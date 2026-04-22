defmodule OpenSleigh.Tracker.Linear do
  @moduledoc """
  Linear-backed implementation of `OpenSleigh.Tracker.Adapter`.

  The adapter is L4 and stateless. The caller supplies all effect
  handles: endpoint, API key, Finch process name, project slug, active
  state names, and the configured `problem_card_ref` source.
  """

  @behaviour OpenSleigh.Tracker.Adapter

  alias OpenSleigh.{EffectError, Ticket}
  alias OpenSleigh.Tracker.Linear.Queries

  @issue_page_size 50
  @comment_page_size 50
  @default_endpoint "https://api.linear.app/graphql"
  @default_finch OpenSleigh.LinearFinch
  @default_problem_card_ref_marker "problem_card_ref"

  @typedoc "Linear adapter handle. Finch itself is started by the L5 caller."
  @type handle :: %{
          required(:api_key) => String.t(),
          required(:project_slug) => String.t(),
          required(:active_states) => [atom() | String.t()],
          optional(:endpoint) => String.t(),
          optional(:finch_name) => atom(),
          optional(:request_fun) => request_fun(),
          optional(:problem_card_ref_field) => String.t(),
          optional(:problem_card_ref_marker) => String.t(),
          optional(:state_name_map) => %{optional(atom()) => String.t()},
          optional(:state_atom_map) => %{optional(String.t()) => atom()}
        }

  @type request_fun ::
          (map(), [{String.t(), String.t()}], handle() ->
             {:ok, %{required(:status) => pos_integer(), required(:body) => binary() | map()}}
             | {:error, term()})

  @impl true
  @spec adapter_kind() :: atom()
  def adapter_kind, do: :linear

  @impl true
  @spec list_active(handle()) :: {:ok, [Ticket.t()]} | {:error, EffectError.t()}
  def list_active(%{} = handle) do
    handle
    |> list_active_state()
    |> fetch_active_pages(handle)
  end

  @impl true
  @spec get(handle(), String.t()) :: {:ok, Ticket.t()} | {:error, EffectError.t()}
  def get(%{} = handle, ticket_id) when is_binary(ticket_id) do
    {query, variables} = Queries.get_issue(ticket_id)

    handle
    |> graphql(query, variables)
    |> decode_single_issue(handle)
  end

  @impl true
  @spec transition(handle(), String.t(), atom()) :: :ok | {:error, EffectError.t()}
  def transition(%{} = handle, ticket_id, new_state)
      when is_binary(ticket_id) and is_atom(new_state) do
    state_name = state_name_for(handle, new_state)

    with {:ok, state_id} <- resolve_state_id(handle, ticket_id, state_name),
         :ok <- update_issue_state(handle, ticket_id, state_id) do
      :ok
    end
  end

  @impl true
  @spec post_comment(handle(), String.t(), String.t()) :: :ok | {:error, EffectError.t()}
  def post_comment(%{} = handle, ticket_id, body)
      when is_binary(ticket_id) and is_binary(body) do
    {query, variables} = Queries.post_comment(ticket_id, body)

    handle
    |> graphql(query, variables)
    |> mutation_success(["data", "commentCreate", "success"])
  end

  @impl true
  @spec list_comments(handle(), String.t()) ::
          {:ok, [OpenSleigh.Tracker.Adapter.normalised_comment()]} | {:error, EffectError.t()}
  def list_comments(%{} = handle, ticket_id) when is_binary(ticket_id) do
    {query, variables} = Queries.list_comments(ticket_id, @comment_page_size)

    handle
    |> graphql(query, variables)
    |> decode_comments()
  end

  @spec list_active_state(handle()) ::
          {:ok, %{project_slug: String.t(), state_names: [String.t()]}}
          | {:error, EffectError.t()}
  defp list_active_state(%{project_slug: project_slug, active_states: active_states} = handle)
       when is_binary(project_slug) and is_list(active_states) do
    state_names =
      active_states
      |> Enum.map(&state_name_for(handle, &1))
      |> Enum.uniq()

    {:ok, %{project_slug: project_slug, state_names: state_names}}
  end

  defp list_active_state(_handle), do: {:error, :tracker_response_malformed}

  @spec fetch_active_pages(
          {:ok, map()} | {:error, EffectError.t()},
          handle()
        ) :: {:ok, [Ticket.t()]} | {:error, EffectError.t()}
  defp fetch_active_pages({:ok, request}, handle) do
    fetch_active_page(handle, request, nil, [])
  end

  defp fetch_active_pages({:error, _reason} = error, _handle), do: error

  @spec fetch_active_page(handle(), map(), String.t() | nil, [Ticket.t()]) ::
          {:ok, [Ticket.t()]} | {:error, EffectError.t()}
  defp fetch_active_page(handle, request, after_cursor, acc) do
    {query, variables} =
      Queries.list_active(
        request.project_slug,
        request.state_names,
        @issue_page_size,
        after_cursor
      )

    with {:ok, body} <- graphql(handle, query, variables),
         {:ok, tickets, page_info} <- decode_issue_page(body, handle) do
      continue_active_pages(handle, request, page_info, tickets ++ acc)
    end
  end

  @spec continue_active_pages(handle(), map(), map(), [Ticket.t()]) ::
          {:ok, [Ticket.t()]} | {:error, EffectError.t()}
  defp continue_active_pages(handle, request, %{has_next_page: true, end_cursor: cursor}, acc)
       when is_binary(cursor) and byte_size(cursor) > 0 do
    fetch_active_page(handle, request, cursor, acc)
  end

  defp continue_active_pages(_handle, _request, %{has_next_page: true}, _acc),
    do: {:error, :tracker_response_malformed}

  defp continue_active_pages(_handle, _request, _page_info, acc) do
    acc
    |> Enum.reverse()
    |> then(&{:ok, &1})
  end

  @spec resolve_state_id(handle(), String.t(), String.t()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp resolve_state_id(handle, ticket_id, state_name) do
    {query, variables} = Queries.state_id(ticket_id, state_name)

    handle
    |> graphql(query, variables)
    |> extract_state_id()
  end

  @spec extract_state_id({:ok, map()} | {:error, EffectError.t()}) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp extract_state_id({:ok, body}) do
    case get_in(body, ["data", "issue", "team", "states", "nodes", Access.at(0), "id"]) do
      state_id when is_binary(state_id) -> {:ok, state_id}
      _ -> {:error, :tracker_response_malformed}
    end
  end

  defp extract_state_id({:error, _reason} = error), do: error

  @spec update_issue_state(handle(), String.t(), String.t()) ::
          :ok | {:error, EffectError.t()}
  defp update_issue_state(handle, ticket_id, state_id) do
    {query, variables} = Queries.transition(ticket_id, state_id)

    handle
    |> graphql(query, variables)
    |> mutation_success(["data", "issueUpdate", "success"])
  end

  @spec mutation_success({:ok, map()} | {:error, EffectError.t()}, [String.t()]) ::
          :ok | {:error, EffectError.t()}
  defp mutation_success({:ok, body}, path) do
    case get_in(body, path) do
      true -> :ok
      _ -> {:error, :tracker_response_malformed}
    end
  end

  defp mutation_success({:error, _reason} = error, _path), do: error

  @spec graphql(handle(), String.t(), map()) :: {:ok, map()} | {:error, EffectError.t()}
  defp graphql(%{} = handle, query, variables) when is_binary(query) and is_map(variables) do
    payload = %{"query" => query, "variables" => variables}
    headers = graphql_headers(handle)

    handle
    |> request_graphql(payload, headers)
    |> decode_graphql_response()
  end

  @spec request_graphql(handle(), map(), [{String.t(), String.t()}]) ::
          {:ok, %{required(:status) => pos_integer(), required(:body) => binary() | map()}}
          | {:error, term()}
  defp request_graphql(%{request_fun: request_fun} = handle, payload, headers)
       when is_function(request_fun, 3) do
    request_fun.(payload, headers, handle)
  end

  defp request_graphql(handle, payload, headers) do
    body = Jason.encode!(payload)
    endpoint = Map.get(handle, :endpoint, @default_endpoint)
    finch_name = Map.get(handle, :finch_name, @default_finch)
    request = Finch.build(:post, endpoint, headers, body)

    case Finch.request(request, finch_name) do
      {:ok, %Finch.Response{status: status, body: response_body}} ->
        {:ok, %{status: status, body: response_body}}

      {:error, reason} ->
        {:error, reason}
    end
  end

  @spec graphql_headers(handle()) :: [{String.t(), String.t()}]
  defp graphql_headers(%{api_key: api_key}) when is_binary(api_key) do
    [
      {"Authorization", api_key},
      {"Content-Type", "application/json"}
    ]
  end

  defp graphql_headers(_handle), do: [{"Content-Type", "application/json"}]

  @spec decode_graphql_response(
          {:ok, %{required(:status) => pos_integer(), required(:body) => binary() | map()}}
          | {:error, term()}
        ) :: {:ok, map()} | {:error, EffectError.t()}
  defp decode_graphql_response({:ok, %{status: 200, body: body}}) do
    body
    |> decode_body()
    |> reject_graphql_errors()
  end

  defp decode_graphql_response({:ok, %{status: _status}}), do: {:error, :tracker_status_non_200}
  defp decode_graphql_response({:error, _reason}), do: {:error, :tracker_request_failed}

  @spec decode_body(binary() | map()) :: {:ok, map()} | {:error, EffectError.t()}
  defp decode_body(body) when is_map(body), do: {:ok, body}

  defp decode_body(body) when is_binary(body) do
    case Jason.decode(body) do
      {:ok, decoded} when is_map(decoded) -> {:ok, decoded}
      _ -> {:error, :tracker_response_malformed}
    end
  end

  defp decode_body(_body), do: {:error, :tracker_response_malformed}

  @spec reject_graphql_errors({:ok, map()} | {:error, EffectError.t()}) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp reject_graphql_errors({:ok, %{"errors" => _errors}}),
    do: {:error, :tracker_response_malformed}

  defp reject_graphql_errors({:ok, body}), do: {:ok, body}
  defp reject_graphql_errors({:error, _reason} = error), do: error

  @spec decode_issue_page({:ok, map()} | map(), handle()) ::
          {:ok, [Ticket.t()], map()} | {:error, EffectError.t()}
  defp decode_issue_page({:ok, body}, handle), do: decode_issue_page(body, handle)

  defp decode_issue_page(
         %{
           "data" => %{
             "issues" => %{
               "nodes" => nodes,
               "pageInfo" => %{"hasNextPage" => has_next_page, "endCursor" => end_cursor}
             }
           }
         },
         handle
       )
       when is_list(nodes) do
    nodes
    |> Enum.reduce_while({:ok, []}, &append_ticket(&1, &2, handle))
    |> attach_page_info(%{has_next_page: has_next_page == true, end_cursor: end_cursor})
  end

  defp decode_issue_page(_body, _handle), do: {:error, :tracker_response_malformed}

  @spec attach_page_info({:ok, [Ticket.t()]} | {:error, EffectError.t()}, map()) ::
          {:ok, [Ticket.t()], map()} | {:error, EffectError.t()}
  defp attach_page_info({:ok, tickets}, page_info), do: {:ok, Enum.reverse(tickets), page_info}
  defp attach_page_info({:error, _reason} = error, _page_info), do: error

  @spec decode_single_issue({:ok, map()} | {:error, EffectError.t()}, handle()) ::
          {:ok, Ticket.t()} | {:error, EffectError.t()}
  defp decode_single_issue({:ok, %{"data" => %{"issue" => issue}}}, handle)
       when is_map(issue) do
    normalize_issue(issue, handle)
  end

  defp decode_single_issue({:ok, _body}, _handle), do: {:error, :tracker_response_malformed}
  defp decode_single_issue({:error, _reason} = error, _handle), do: error

  @spec decode_comments({:ok, map()} | {:error, EffectError.t()}) ::
          {:ok, [OpenSleigh.Tracker.Adapter.normalised_comment()]} | {:error, EffectError.t()}
  defp decode_comments({
         :ok,
         %{"data" => %{"issue" => %{"comments" => %{"nodes" => nodes}}}}
       })
       when is_list(nodes) do
    nodes
    |> Enum.map(&normalise_comment/1)
    |> Enum.reject(&is_nil/1)
    |> then(&{:ok, &1})
  end

  defp decode_comments({:ok, _body}), do: {:error, :tracker_response_malformed}
  defp decode_comments({:error, _reason} = error), do: error

  @spec normalise_comment(map()) :: OpenSleigh.Tracker.Adapter.normalised_comment() | nil
  defp normalise_comment(%{"id" => id, "body" => body} = comment)
       when is_binary(id) and is_binary(body) do
    %{
      id: id,
      body: body,
      author: comment_author(comment),
      created_at: parse_datetime(comment["createdAt"]),
      url: comment["url"]
    }
  end

  defp normalise_comment(_comment), do: nil

  @spec comment_author(map()) :: String.t()
  defp comment_author(%{"user" => %{"email" => email}}) when is_binary(email) and email != "",
    do: email

  defp comment_author(%{"user" => %{"name" => name}}) when is_binary(name) and name != "",
    do: name

  defp comment_author(_comment), do: "unknown"

  @spec append_ticket(map(), {:ok, [Ticket.t()]} | {:error, EffectError.t()}, handle()) ::
          {:cont, {:ok, [Ticket.t()]}} | {:halt, {:error, EffectError.t()}}
  defp append_ticket(issue, {:ok, tickets}, handle) do
    case normalize_issue(issue, handle) do
      {:ok, ticket} -> {:cont, {:ok, [ticket | tickets]}}
      {:error, reason} -> {:halt, {:error, reason}}
    end
  end

  defp append_ticket(_issue, {:error, _reason} = error, _handle), do: {:halt, error}

  @spec normalize_issue(map(), handle()) :: {:ok, Ticket.t()} | {:error, EffectError.t()}
  defp normalize_issue(issue, handle) when is_map(issue) do
    issue
    |> ticket_attrs(handle)
    |> Ticket.new()
    |> map_ticket_result()
  end

  @spec ticket_attrs(map(), handle()) :: map()
  defp ticket_attrs(issue, handle) do
    %{
      id: issue["id"],
      source: {:linear, handle.project_slug},
      title: issue["title"],
      body: Map.get(issue, "description", ""),
      state: state_atom_for(handle, get_in(issue, ["state", "name"])),
      problem_card_ref: problem_card_ref(issue, handle),
      target_branch: issue["branchName"],
      fetched_at: DateTime.utc_now(),
      metadata: issue_metadata(issue)
    }
  end

  @spec map_ticket_result({:ok, Ticket.t()} | {:error, term()}) ::
          {:ok, Ticket.t()} | {:error, EffectError.t()}
  defp map_ticket_result({:ok, %Ticket{} = ticket}), do: {:ok, ticket}
  defp map_ticket_result({:error, _reason}), do: {:error, :tracker_response_malformed}

  @spec problem_card_ref(map(), handle()) :: String.t() | nil
  defp problem_card_ref(issue, %{problem_card_ref_field: field} = handle) when is_binary(field) do
    issue
    |> Map.get(field)
    |> normalize_problem_card_ref()
    |> fallback_problem_card_ref(issue, problem_card_ref_marker(handle))
  end

  defp problem_card_ref(issue, handle) do
    marker = problem_card_ref_marker(handle)

    issue
    |> Map.get("description", "")
    |> extract_markdown_marker(marker)
  end

  @spec fallback_problem_card_ref(String.t() | nil, map(), String.t()) :: String.t() | nil
  defp fallback_problem_card_ref(nil, issue, marker) do
    issue
    |> Map.get("description", "")
    |> extract_markdown_marker(marker)
  end

  defp fallback_problem_card_ref(ref, _issue, _marker), do: ref

  @spec problem_card_ref_marker(handle()) :: String.t()
  defp problem_card_ref_marker(handle) do
    Map.get(handle, :problem_card_ref_marker, @default_problem_card_ref_marker)
  end

  @spec extract_markdown_marker(term(), String.t()) :: String.t() | nil
  defp extract_markdown_marker(description, marker)
       when is_binary(description) and is_binary(marker) do
    marker_pattern = Regex.escape(marker)
    regex = ~r/^\s*#{marker_pattern}\s*:\s*(\S+)\s*$/im

    case Regex.run(regex, description) do
      [_match, ref] -> normalize_problem_card_ref(ref)
      _ -> nil
    end
  end

  defp extract_markdown_marker(_description, _marker), do: nil

  @spec normalize_problem_card_ref(term()) :: String.t() | nil
  defp normalize_problem_card_ref(value) when is_binary(value) do
    value
    |> String.trim()
    |> blank_to_nil()
  end

  defp normalize_problem_card_ref(_value), do: nil

  @spec blank_to_nil(String.t()) :: String.t() | nil
  defp blank_to_nil(""), do: nil
  defp blank_to_nil(value), do: value

  @spec issue_metadata(map()) :: map()
  defp issue_metadata(issue) do
    %{
      linear_identifier: issue["identifier"],
      linear_url: issue["url"],
      linear_state_name: get_in(issue, ["state", "name"]),
      linear_project_slug: get_in(issue, ["project", "slugId"]),
      linear_created_at: issue["createdAt"],
      linear_updated_at: issue["updatedAt"]
    }
  end

  @spec parse_datetime(term()) :: DateTime.t() | nil
  defp parse_datetime(raw) when is_binary(raw) do
    case DateTime.from_iso8601(raw) do
      {:ok, dt, _offset} -> dt
      _ -> nil
    end
  end

  defp parse_datetime(_raw), do: nil

  @spec state_name_for(handle(), atom() | String.t()) :: String.t()
  defp state_name_for(_handle, state) when is_binary(state), do: state

  defp state_name_for(handle, state) when is_atom(state) do
    handle
    |> Map.get(:state_name_map, %{})
    |> Map.get(state)
    |> fallback_state_name(state)
  end

  @spec fallback_state_name(String.t() | nil, atom()) :: String.t()
  defp fallback_state_name(nil, state), do: humanize_atom(state)
  defp fallback_state_name(name, _state), do: name

  @spec state_atom_for(handle(), String.t() | nil) :: atom()
  defp state_atom_for(handle, state_name) when is_binary(state_name) do
    handle
    |> Map.get(:state_atom_map, %{})
    |> Map.get(state_name)
    |> fallback_state_atom(state_name)
  end

  defp state_atom_for(_handle, _state_name), do: :other

  @spec fallback_state_atom(atom() | nil, String.t()) :: atom()
  defp fallback_state_atom(nil, state_name), do: known_state_atom(state_name)
  defp fallback_state_atom(state, _state_name), do: state

  @spec known_state_atom(String.t()) :: atom()
  defp known_state_atom(state_name) do
    state_name
    |> String.downcase()
    |> String.trim()
    |> known_state_atom_from_normalized()
  end

  @spec known_state_atom_from_normalized(String.t()) :: atom()
  defp known_state_atom_from_normalized("todo"), do: :todo
  defp known_state_atom_from_normalized("to do"), do: :todo
  defp known_state_atom_from_normalized("backlog"), do: :todo
  defp known_state_atom_from_normalized("in progress"), do: :in_progress
  defp known_state_atom_from_normalized("started"), do: :in_progress
  defp known_state_atom_from_normalized("done"), do: :done
  defp known_state_atom_from_normalized("completed"), do: :done
  defp known_state_atom_from_normalized("canceled"), do: :canceled
  defp known_state_atom_from_normalized("cancelled"), do: :canceled
  defp known_state_atom_from_normalized(_state_name), do: :other

  @spec humanize_atom(atom()) :: String.t()
  defp humanize_atom(state) do
    state
    |> Atom.to_string()
    |> String.split("_")
    |> Enum.map_join(" ", &String.capitalize/1)
  end
end
