defmodule OpenSleigh.Judge.AgentInvokerTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.{Agent.Mock, ConfigHash}
  alias OpenSleigh.Judge.AgentInvoker

  setup do
    Mock.reset!()

    workspace =
      System.tmp_dir!()
      |> Path.join("open_sleigh_judge_invoker_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace)

    on_exit(fn ->
      Mock.reset!()
      File.rm_rf!(workspace)
    end)

    %{workspace: workspace}
  end

  test "invokes an Agent.Adapter session and decodes JSON text", ctx do
    Mock.put_turn_replies([
      %{
        text: ~s({"verdict":"pass","cl":3,"rationale":"specific artifact"})
      }
    ])

    config =
      AgentInvoker.config(
        Mock,
        ctx.workspace,
        ConfigHash.from_iodata("judge-test"),
        %{}
      )

    assert {:ok, response} = AgentInvoker.invoke("Judge this", config)
    assert response["verdict"] == "pass"
    assert response["cl"] == 3
    assert Mock.start_count() == 1
  end

  test "extracts JSON from fenced or narrated adapter output", ctx do
    Mock.put_turn_replies([
      %{
        text: """
        ```json
        {"verdict":"fail","cl":1,"rationale":"vague"}
        ```
        """
      }
    ])

    config =
      AgentInvoker.config(
        Mock,
        ctx.workspace,
        ConfigHash.from_iodata("judge-test"),
        %{}
      )

    assert {:ok, response} = AgentInvoker.invoke("Judge this", config)
    assert response["verdict"] == "fail"
    assert response["rationale"] == "vague"
  end

  test "returns malformed when adapter output is not a JSON object", ctx do
    Mock.put_turn_replies([%{text: "not json"}])

    config =
      AgentInvoker.config(
        Mock,
        ctx.workspace,
        ConfigHash.from_iodata("judge-test"),
        %{}
      )

    assert {:error, :judge_response_malformed} = AgentInvoker.invoke("Judge this", config)
  end
end
