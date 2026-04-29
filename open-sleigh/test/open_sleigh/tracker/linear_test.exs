defmodule OpenSleigh.Tracker.LinearTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Tracker.Linear
  alias OpenSleigh.Tracker.Linear.Queries

  test "query builders keep Linear operations and variables explicit" do
    {query, variables} = Queries.list_active("oct", ["Todo", "In Progress"], 50, nil)

    assert query =~ "OpenSleighLinearListActive"
    assert query =~ "projectSlug"
    assert query =~ "stateNames"
    assert variables["projectSlug"] == "oct"
    assert variables["stateNames"] == ["Todo", "In Progress"]

    {query, variables} = Queries.transition("issue-1", "state-1")
    assert query =~ "issueUpdate"
    assert variables == %{"issueId" => "issue-1", "stateId" => "state-1"}
  end

  test "list_active normalizes Linear issues into Tickets with configured marker" do
    handle = handle_with_responses([list_response([linear_issue(%{})])])

    assert {:ok, [ticket]} = Linear.list_active(handle)
    assert ticket.id == "issue-1"
    assert ticket.source == {:linear, "oct"}
    assert ticket.state == :in_progress
    assert ticket.problem_card_ref == "haft-pc-1"
    assert ticket.metadata.linear_identifier == "OCT-1"
  end

  test "list_active supports a direct payload field for future custom-field projection" do
    issue =
      %{"problemCardRef" => "haft-pc-custom"}
      |> linear_issue()
      |> Map.put("description", "")

    handle =
      [list_response([issue])]
      |> handle_with_responses()
      |> Map.put(:problem_card_ref_field, "problemCardRef")

    assert {:ok, [ticket]} = Linear.list_active(handle)
    assert ticket.problem_card_ref == "haft-pc-custom"
  end

  test "get fetches and normalizes one Linear issue" do
    handle = handle_with_responses([single_response(linear_issue(%{"id" => "issue-2"}))])

    assert {:ok, ticket} = Linear.get(handle, "issue-2")
    assert ticket.id == "issue-2"
    assert ticket.problem_card_ref == "haft-pc-1"
  end

  test "transition resolves state id then updates issue state" do
    handle =
      handle_with_responses([
        %{
          "data" => %{
            "issue" => %{
              "team" => %{"states" => %{"nodes" => [%{"id" => "state-done"}]}}
            }
          }
        },
        %{"data" => %{"issueUpdate" => %{"success" => true}}}
      ])

    assert :ok = Linear.transition(handle, "issue-1", :done)

    queries = sent_queries(handle)
    assert Enum.at(queries, 0)["variables"]["stateName"] == "Done"
    assert Enum.at(queries, 1)["variables"]["stateId"] == "state-done"
  end

  test "post_comment creates a Linear comment" do
    handle = handle_with_responses([%{"data" => %{"commentCreate" => %{"success" => true}}}])

    assert :ok = Linear.post_comment(handle, "issue-1", "Open-Sleigh blocked: missing frame")

    [payload] = sent_queries(handle)
    assert payload["variables"]["body"] == "Open-Sleigh blocked: missing frame"
    assert payload["query"] =~ "commentCreate"
  end

  test "list_comments normalizes Linear comments for HumanGateListener" do
    response = %{
      "data" => %{
        "issue" => %{
          "comments" => %{
            "nodes" => [
              %{
                "id" => "comment-1",
                "body" => "/approve",
                "createdAt" => "2026-04-22T10:00:00.000Z",
                "url" => "https://linear.app/comment-1",
                "user" => %{"email" => "ivan@example.com", "name" => "Ivan"}
              }
            ]
          }
        }
      }
    }

    handle = handle_with_responses([response])

    assert {:ok, [comment]} = Linear.list_comments(handle, "issue-1")
    assert comment.id == "comment-1"
    assert comment.body == "/approve"
    assert comment.author == "ivan@example.com"
    assert comment.url == "https://linear.app/comment-1"
  end

  test "maps request and response failures into closed EffectError atoms" do
    request_failed =
      base_handle()
      |> Map.put(:request_fun, fn _payload, _headers, _handle -> {:error, :nxdomain} end)

    non_200 = handle_with_raw_response(%{status: 500, body: "{}"})
    malformed = handle_with_raw_response(%{status: 200, body: "not json"})

    graphql_errors =
      handle_with_raw_response(%{status: 200, body: %{"errors" => [%{"message" => "x"}]}})

    assert {:error, :tracker_request_failed} = Linear.list_active(request_failed)
    assert {:error, :tracker_status_non_200} = Linear.list_active(non_200)
    assert {:error, :tracker_response_malformed} = Linear.list_active(malformed)
    assert {:error, :tracker_response_malformed} = Linear.list_active(graphql_errors)
  end

  @tag :integration
  test "real Linear API lists configured project issues when env is present" do
    with {:ok, api_key} <- System.fetch_env("LINEAR_API_KEY"),
         {:ok, project_slug} <- System.fetch_env("LINEAR_PROJECT_SLUG") do
      {:ok, _finch} = start_supervised({Finch, name: OpenSleigh.LinearFinch})

      handle = %{
        api_key: api_key,
        project_slug: project_slug,
        active_states: ["Todo", "In Progress"],
        problem_card_ref_marker:
          System.get_env("OPEN_SLEIGH_LINEAR_PROBLEM_CARD_MARKER") || "problem_card_ref"
      }

      assert {:ok, tickets} = Linear.list_active(handle)
      assert is_list(tickets)
    else
      _missing_env -> :ok
    end
  end

  defp base_handle do
    %{
      api_key: "lin_api_test",
      project_slug: "oct",
      active_states: [:todo, :in_progress],
      state_name_map: %{todo: "Todo", in_progress: "In Progress", done: "Done"},
      state_atom_map: %{"Todo" => :todo, "In Progress" => :in_progress, "Done" => :done}
    }
  end

  defp handle_with_responses(responses) do
    {:ok, agent} = Agent.start_link(fn -> %{responses: responses, sent: []} end)

    base_handle()
    |> Map.put(:request_fun, fn payload, headers, _handle ->
      Agent.get_and_update(agent, fn state ->
        [response | rest] = state.responses
        sent = [%{"payload" => payload, "headers" => headers} | state.sent]
        {{:ok, %{status: 200, body: response}}, %{state | responses: rest, sent: sent}}
      end)
    end)
    |> Map.put(:test_agent, agent)
  end

  defp handle_with_raw_response(response) do
    base_handle()
    |> Map.put(:request_fun, fn _payload, _headers, _handle -> {:ok, response} end)
  end

  defp sent_queries(%{test_agent: agent}) do
    agent
    |> Agent.get(&Enum.reverse(&1.sent))
    |> Enum.map(& &1["payload"])
  end

  defp list_response(nodes) do
    %{
      "data" => %{
        "issues" => %{
          "nodes" => nodes,
          "pageInfo" => %{"hasNextPage" => false, "endCursor" => nil}
        }
      }
    }
  end

  defp single_response(issue) do
    %{"data" => %{"issue" => issue}}
  end

  defp linear_issue(overrides) do
    defaults = %{
      "id" => "issue-1",
      "identifier" => "OCT-1",
      "title" => "Add rate limiter",
      "description" => "problem_card_ref: haft-pc-1\n\nImplement the limiter.",
      "state" => %{"name" => "In Progress"},
      "branchName" => "feature/rate-limiter",
      "url" => "https://linear.app/oct/issue/OCT-1/add-rate-limiter",
      "project" => %{"slugId" => "oct", "name" => "Oct"},
      "createdAt" => "2026-04-22T10:00:00.000Z",
      "updatedAt" => "2026-04-22T10:01:00.000Z"
    }

    Map.merge(defaults, overrides)
  end
end
