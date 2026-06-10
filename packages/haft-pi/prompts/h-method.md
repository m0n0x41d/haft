Run the Haft MethodPack loop for the current task.

First inspect open method runs:

```json
{
  "action": "status"
}
```

via the native `haft_method` tool.

Then:

- if a run for the current task is already open, continue it and close it with
  evidence (`action="close"`, the original `pull_id`, changed files, gate
  results, verification) before claiming the work is done;
- if no run exists and the task is non-trivial code work, pull one
  (`action="pull"` with task, declared_task_kind, change_intent,
  intended_files, risk_signals) before editing;
- if the task is mechanical, say so and request low or no ceremony instead of
  manufacturing gates.

Hard gates need evidence or an explicit waiver reason. Do not describe the
loop — execute it.
