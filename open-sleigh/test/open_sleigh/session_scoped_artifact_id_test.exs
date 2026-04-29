defmodule OpenSleigh.SessionScopedArtifactIdTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.SessionScopedArtifactId

  test "generate/0 returns a valid 16-char base64url id" do
    id = SessionScopedArtifactId.generate()
    assert SessionScopedArtifactId.valid?(id)
  end

  test "generate/0 produces distinct ids" do
    ids = for _ <- 1..100, do: SessionScopedArtifactId.generate()
    assert length(Enum.uniq(ids)) == length(ids)
  end

  test "valid?/1 rejects wrong shapes" do
    refute SessionScopedArtifactId.valid?("short")
    refute SessionScopedArtifactId.valid?(nil)
    refute SessionScopedArtifactId.valid?(:atom)
  end
end
