defmodule OpenSleigh.MixProject do
  use Mix.Project

  def project do
    [
      app: :open_sleigh,
      version: "0.1.0",
      elixir: "~> 1.18",
      start_permanent: Mix.env() == :prod,
      elixirc_options: [warnings_as_errors: true],
      elixirc_paths: elixirc_paths(Mix.env()),
      deps: deps(),
      dialyzer: dialyzer(),
      escript: [main_module: OpenSleigh.CLI, name: "open_sleigh"],
      releases: releases(),
      test_coverage: [tool: ExCoveralls],
      preferred_cli_env: [
        coveralls: :test,
        "coveralls.detail": :test,
        "coveralls.html": :test
      ]
    ]
  end

  defp elixirc_paths(:test), do: ["lib", "test/support"]
  defp elixirc_paths(_), do: ["lib"]

  def application do
    [
      extra_applications: extra_applications(Mix.env()),
      mod: {OpenSleigh.Application, []}
    ]
  end

  defp extra_applications(_env), do: [:logger, :mix]

  defp releases do
    [
      open_sleigh: [
        include_executables_for: [:unix],
        applications: [runtime_tools: :permanent]
      ]
    ]
  end

  # Dependency policy — see specs/enabling-system/STACK_DECISION.md.
  # MVP-1 minimum. Adding a dep requires an ADR justifying why a
  # 200-line implementation won't do (Thai-disaster prevention).
  defp deps do
    [
      # L1–L5 test tooling.
      {:stream_data, "~> 1.1", only: [:dev, :test]},
      {:excoveralls, "~> 0.18", only: :test, runtime: false},
      # Static analysis — enforces the four-label taxonomy in code.
      {:credo, "~> 1.7", only: [:dev, :test], runtime: false},
      {:dialyxir, "~> 1.4", only: [:dev, :test], runtime: false},
      # L4 deps (wire up when the layer lands).
      {:jason, "~> 1.4"},
      {:finch, "~> 0.18"},
      # L6 compiler deps.
      {:yaml_elixir, "~> 2.11"},
      {:earmark_parser, "~> 1.4"}
      # L4/L6 deps still gated by their layers landing:
      # {:telemetry, "~> 1.3"}         # ObservationsBus transport
    ]
  end

  defp dialyzer do
    [
      plt_add_apps: [:ex_unit, :mix],
      plt_core_path: "priv/plts",
      plt_local_path: "priv/plts",
      flags: [:error_handling, :underspecs, :unknown, :missing_return, :extra_return]
    ]
  end
end
