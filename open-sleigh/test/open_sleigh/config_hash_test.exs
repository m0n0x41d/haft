defmodule OpenSleigh.ConfigHashTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.ConfigHash

  test "from_iodata/1 returns 64-char lowercase hex" do
    h = ConfigHash.from_iodata("payload")
    assert is_binary(h) and byte_size(h) == 64
    assert String.match?(h, ~r/^[0-9a-f]{64}$/)
  end

  test "from_iodata/1 is deterministic" do
    assert ConfigHash.from_iodata("a") == ConfigHash.from_iodata("a")
    refute ConfigHash.from_iodata("a") == ConfigHash.from_iodata("b")
  end

  test "new/1 accepts valid 64-char hex" do
    {:ok, h} = ConfigHash.new(String.duplicate("0", 64))
    assert ConfigHash.valid?(h)
  end

  test "new/1 rejects wrong length" do
    assert {:error, :invalid_length} = ConfigHash.new(String.duplicate("0", 63))
    assert {:error, :invalid_length} = ConfigHash.new("")
  end

  test "new/1 rejects non-hex" do
    assert {:error, :not_hex} = ConfigHash.new(String.duplicate("g", 64))
    assert {:error, :not_hex} = ConfigHash.new(String.duplicate("A", 64))
  end

  test "valid?/1 rejects non-binaries" do
    refute ConfigHash.valid?(nil)
    refute ConfigHash.valid?(42)
    refute ConfigHash.valid?(:atom)
  end
end
