defmodule OpenSleigh.EffectError do
  @moduledoc """
  Closed sum of every expected failure mode across all L4 adapters.

  Per `specs/target-system/AGENT_PROTOCOL.md §6` + `ILLEGAL_STATES.md`
  AD1 / AD2: adapter callbacks NEVER return an untyped error and NEVER
  raise for a known failure mode. Adding a new variant requires
  extending this sum — enforced by `mix credo` + Dialyzer.

  Variants map directly to the taxonomy in AGENT_PROTOCOL.md §6; this
  module is the carrier. Callers pattern-match on the atom; open-set
  extension (e.g. via `atom()`) is forbidden.
  """

  @typedoc "The closed failure-mode alphabet."
  @type t ::
          :agent_command_not_found
          | :agent_launch_failed
          | :invalid_workspace_cwd
          | :handshake_timeout
          | :initialize_failed
          | :thread_start_failed
          | :turn_start_failed
          | :turn_timeout
          | :stall_timeout
          | :turn_input_required
          | :port_exit_unexpected
          | :response_parse_error
          | :tool_forbidden_by_phase_scope
          | :tool_unknown_to_adapter
          | :tool_arg_invalid
          | :tool_execution_failed
          | :haft_unavailable
          | :rate_limit_exceeded
          | :unsupported_event_category
          | :tracker_request_failed
          | :tracker_status_non_200
          | :tracker_response_malformed
          | :commission_not_found
          | :commission_not_runnable
          | :commission_lock_conflict
          | :judge_unavailable
          | :judge_response_malformed
          | :uncalibrated
          | :path_outside_workspace
          | :path_symlink_escape
          | :path_hardlink_escape
          | :path_symlink_loop
          | :workspace_is_self
          | :path_forbidden
          | :mutation_outside_commission_scope
          | :cancel_grace_expired

  @all [
    :agent_command_not_found,
    :agent_launch_failed,
    :invalid_workspace_cwd,
    :handshake_timeout,
    :initialize_failed,
    :thread_start_failed,
    :turn_start_failed,
    :turn_timeout,
    :stall_timeout,
    :turn_input_required,
    :port_exit_unexpected,
    :response_parse_error,
    :tool_forbidden_by_phase_scope,
    :tool_unknown_to_adapter,
    :tool_arg_invalid,
    :tool_execution_failed,
    :haft_unavailable,
    :rate_limit_exceeded,
    :unsupported_event_category,
    :tracker_request_failed,
    :tracker_status_non_200,
    :tracker_response_malformed,
    :commission_not_found,
    :commission_not_runnable,
    :commission_lock_conflict,
    :judge_unavailable,
    :judge_response_malformed,
    :uncalibrated,
    :path_outside_workspace,
    :path_symlink_escape,
    :path_hardlink_escape,
    :path_symlink_loop,
    :workspace_is_self,
    :path_forbidden,
    :mutation_outside_commission_scope,
    :cancel_grace_expired
  ]

  @doc "Every admissible error atom."
  @spec all() :: [t(), ...]
  def all, do: @all

  @doc "Is `value` a valid `EffectError`?"
  @spec valid?(term()) :: boolean()
  def valid?(value) when value in @all, do: true
  def valid?(_), do: false
end
