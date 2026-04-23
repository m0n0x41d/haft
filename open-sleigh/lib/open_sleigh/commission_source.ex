defmodule OpenSleigh.CommissionSource do
  @moduledoc """
  Behaviour contract for commission-source adapters.

  Commission sources are intake adapters only. They may list WorkCommission
  snapshots that are eligible for Preflight and claim one snapshot for
  Preflight, but they do not create, approve, or complete commissions.
  """

  alias OpenSleigh.{Scope, WorkCommission}

  @typedoc "Per-implementation handle, usually compiled from `sleigh.md` config."
  @type handle :: term()

  @typedoc "Expected commission-source adapter failure modes."
  @type source_error ::
          :fixture_path_missing
          | :fixture_path_invalid
          | :fixture_read_failed
          | :fixture_parse_failed
          | :fixture_payload_invalid
          | :commission_source_invalid
          | :commission_response_malformed
          | :commission_not_found
          | :commission_not_runnable
          | :commission_lock_conflict
          | :haft_unavailable
          | :response_parse_error
          | :tool_execution_failed
          | Scope.new_error()
          | WorkCommission.new_error()

  @doc "Return WorkCommission snapshots that are eligible for Preflight."
  @callback list_runnable(handle()) ::
              {:ok, [WorkCommission.t()]} | {:error, source_error()}

  @doc "Claim the first runnable WorkCommission for Preflight."
  @callback claim_for_preflight(handle()) ::
              {:ok, WorkCommission.t()} | {:error, source_error()}

  @doc "Claim one runnable WorkCommission by id for Preflight."
  @callback claim_for_preflight(handle(), commission_id :: String.t()) ::
              {:ok, WorkCommission.t()} | {:error, source_error()}

  @doc "Adapter kind atom (`:local`, `:haft`, etc.)."
  @callback adapter_kind() :: atom()
end
