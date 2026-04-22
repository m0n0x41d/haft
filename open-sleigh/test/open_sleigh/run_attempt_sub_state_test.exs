defmodule OpenSleigh.RunAttemptSubStateTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.RunAttemptSubState

  test "all/0 contains the eleven Symphony-inherited sub-states" do
    assert length(RunAttemptSubState.all()) == 11
  end

  test "terminal?/1 distinguishes terminal sub-states" do
    for sub <- [:succeeded, :failed, :timed_out, :stalled, :canceled_by_reconciliation] do
      assert RunAttemptSubState.terminal?(sub)
    end

    for sub <- [:preparing_workspace, :building_prompt, :streaming_turn, :finishing] do
      refute RunAttemptSubState.terminal?(sub)
    end
  end

  test "retryable?/1 selects failure-class terminals" do
    assert RunAttemptSubState.retryable?(:failed)
    assert RunAttemptSubState.retryable?(:timed_out)
    assert RunAttemptSubState.retryable?(:stalled)
  end

  test "retryable?/1 refuses :succeeded and :canceled_by_reconciliation" do
    refute RunAttemptSubState.retryable?(:succeeded)
    refute RunAttemptSubState.retryable?(:canceled_by_reconciliation)
  end

  test "valid?/1 rejects unknown sub-states" do
    refute RunAttemptSubState.valid?(:unknown)
    refute RunAttemptSubState.valid?(:running)
    refute RunAttemptSubState.valid?(nil)
  end
end
