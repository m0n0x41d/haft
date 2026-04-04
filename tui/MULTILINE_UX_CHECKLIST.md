# Multiline Input UX Checklist

Reference behavior for prompt editing should stay aligned with the multiline-first terminal flow used in `.context/repos/claude-code`, especially its paste handling and full prompt transcript rendering.

- Large bracketed paste: paste 10+ wrapped lines and confirm the whole prompt stays visible, wrapped, and cursor-visible before submit.
- Typed multiline input: use `Ctrl+J` to insert newlines, then verify `Home`, `End`, arrows, and submit still operate on the expected line/cursor position.
- Image paste plus typing: paste or attach an image, keep typing multiline text, and confirm the attachment strip stays above the prompt without hiding the text entry area.
- Queued or history edit: submit a multiline prompt, recall it with the input history path, and confirm line breaks survive round-trip editing.
- Transcript rendering: after submit, confirm chat history shows every prompt line verbatim and does not reinterpret bracket-prefixed lines as attachments.
