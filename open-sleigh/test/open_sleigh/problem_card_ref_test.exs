defmodule OpenSleigh.ProblemCardRefTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.ProblemCardRef

  test "new/1 wraps non-empty binary" do
    assert {:ok, "haft-pc-abc"} = ProblemCardRef.new("haft-pc-abc")
  end

  test "new/1 rejects empty or non-binary" do
    assert {:error, :empty_or_invalid} = ProblemCardRef.new("")
    assert {:error, :empty_or_invalid} = ProblemCardRef.new(nil)
    assert {:error, :empty_or_invalid} = ProblemCardRef.new(42)
  end

  test "valid?/1 shape check" do
    assert ProblemCardRef.valid?("haft-pc-abc")
    refute ProblemCardRef.valid?("")
    refute ProblemCardRef.valid?(nil)
  end
end
