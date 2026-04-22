defmodule OpenSleigh.SessionIdTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.SessionId

  test "generate/0 returns a valid 22-char base64url id" do
    id = SessionId.generate()
    assert SessionId.valid?(id)
  end

  test "generate/0 produces different ids on each call" do
    ids = for _ <- 1..100, do: SessionId.generate()
    assert length(Enum.uniq(ids)) == length(ids)
  end

  test "new/1 accepts canonical shape" do
    id = SessionId.generate()
    assert {:ok, ^id} = SessionId.new(id)
  end

  test "new/1 rejects wrong length" do
    assert {:error, :invalid_format} = SessionId.new(String.duplicate("a", 21))
    assert {:error, :invalid_format} = SessionId.new("")
    assert {:error, :invalid_format} = SessionId.new(nil)
  end

  test "new/1 rejects non-base64url chars" do
    assert {:error, :invalid_format} = SessionId.new(String.duplicate("!", 22))
  end
end
