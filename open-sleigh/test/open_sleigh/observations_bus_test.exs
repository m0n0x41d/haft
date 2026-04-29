defmodule OpenSleigh.ObservationsBusTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.ObservationsBus

  setup do
    ObservationsBus.reset()
    :ok
  end

  describe "emit/3 — OB3 type-narrowing" do
    test "accepts numbers" do
      assert :ok = ObservationsBus.emit(:gate_bypass_rate, 0.15)
      assert :ok = ObservationsBus.emit(:retry_count, 3)
    end

    test "accepts binaries and atoms" do
      assert :ok = ObservationsBus.emit(:last_event, "turn_completed")
      assert :ok = ObservationsBus.emit(:sub_state, :streaming_turn)
    end

    test "rejects maps / tuples / structs at clause level" do
      assert_raise FunctionClauseError, fn ->
        ObservationsBus.emit(:bad, %{not: :scalar})
      end

      assert_raise FunctionClauseError, fn ->
        ObservationsBus.emit(:bad, {:tuple, :value})
      end
    end

    test "emits with tags" do
      assert :ok = ObservationsBus.emit(:gate_bypass_rate, 0.2, %{phase: :execute})
      snapshot = ObservationsBus.snapshot()
      assert Enum.any?(snapshot, &(&1.tags == %{phase: :execute}))
    end
  end

  describe "snapshot/0" do
    test "returns empty list when no observations" do
      assert [] == ObservationsBus.snapshot()
    end

    test "returns list of {metric, value, tags, at}" do
      :ok = ObservationsBus.emit(:x, 1)
      [obs] = ObservationsBus.snapshot()
      assert obs.metric == :x
      assert obs.value == 1
      assert is_integer(obs.at)
    end

    test "overwrites same-key observations" do
      :ok = ObservationsBus.emit(:x, 1)
      :ok = ObservationsBus.emit(:x, 2)
      [obs] = ObservationsBus.snapshot()
      assert obs.value == 2
    end
  end

  describe "OB1/OB5 — zero compile-time call to any Haft module" do
    test "beam imports chunk contains no OpenSleigh.Haft.* MFAs" do
      # The .beam `imports` chunk lists every external MFA the module
      # calls. If ObservationsBus ever gained a call to any Haft
      # module (direct or via alias), it would appear here.
      path = :code.which(OpenSleigh.ObservationsBus)
      {:ok, {_mod, [imports: imports]}} = :beam_lib.chunks(path, [:imports])

      haft_imports =
        Enum.filter(imports, fn {mod, _fun, _arity} ->
          mod |> Atom.to_string() |> String.starts_with?("Elixir.OpenSleigh.Haft")
        end)

      assert haft_imports == [],
             "OB1/OB5 violation: ObservationsBus imports from Haft modules: " <>
               inspect(haft_imports)
    end

    test "source file has no alias/import/use of Haft" do
      # Catches the case where a future addition aliases Haft but
      # only uses it inside a dead-code path the compiler stripped.
      source = File.read!("lib/open_sleigh/observations_bus.ex")
      refute source =~ ~r/^\s*alias\s+OpenSleigh\.Haft/m
      refute source =~ ~r/^\s*import\s+OpenSleigh\.Haft/m
      refute source =~ ~r/^\s*use\s+OpenSleigh\.Haft/m
    end
  end
end
