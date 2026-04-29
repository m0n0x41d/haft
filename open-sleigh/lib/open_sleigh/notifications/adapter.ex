defmodule OpenSleigh.Notifications.Adapter do
  @moduledoc """
  Abstract port for human-facing notifications.

  Runtime code can emit the same closed notification shape to a local log,
  Slack-like team channel, or another future adapter without changing the
  orchestrator state model.
  """

  @type kind :: :human_gate_pending | :blocking_failure

  @type notification :: %{
          required(:kind) => kind(),
          required(:message) => String.t(),
          required(:metadata) => map()
        }

  @type handle :: term()

  @callback notify(handle(), notification()) :: :ok | {:error, atom()}

  @doc "Validate the portable notification shape."
  @spec valid?(term()) :: boolean()
  def valid?(%{kind: kind, message: message, metadata: metadata})
      when kind in [:human_gate_pending, :blocking_failure] and is_binary(message) and
             is_map(metadata),
      do: true

  def valid?(_value), do: false
end
