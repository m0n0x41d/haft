include_integration? =
  [
    System.get_env("OPEN_SLEIGH_INCLUDE_INTEGRATION"),
    System.get_env("CODEX_CMD"),
    System.get_env("LINEAR_API_KEY")
  ]
  |> Enum.reject(&is_nil/1)
  |> Enum.reject(&(&1 == ""))
  |> Enum.any?()

exclude =
  case include_integration? do
    true -> []
    false -> [integration: true]
  end

ExUnit.configure(exclude: exclude)
ExUnit.start()
