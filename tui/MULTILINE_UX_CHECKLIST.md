# Multiline Input UX Checklist

Reference behavior for prompt editing should stay aligned with the multiline-first terminal flow used in `.context/repos/claude-code`, especially its paste handling and full prompt transcript rendering.

Automated guardrails:
- `src/terminal/inputStream.multiline.test.ts` protects bracketed paste assembly and mouse-sequence stripping.
- `src/components/multilineUx.test.ts` protects the multiline prompt contract across paste, wrapped layout, attachments, history, and transcript rendering.
- `src/input/editBuffer.test.ts`, `src/input/history.test.ts`, and `src/components/userPrompt.test.ts` keep multiline cursor movement, recall, and display text lossless.

Verification command:
- `bun test src/terminal src/input src/components && bun run typecheck`

- Large bracketed paste: paste 10+ wrapped lines and confirm the whole prompt stays visible, wrapped, and cursor-visible before submit.
- Typed multiline input: use `Ctrl+J` to insert newlines, then verify `Home`, `End`, and `↑/↓` keep the cursor inside the prompt until the first/last line, after which history navigation may take over.
- Image paste plus typing: paste or attach an image, keep typing multiline text, and confirm the attachment strip stays above the prompt without hiding the text entry area.
- Queued or history edit: submit a multiline prompt, recall it with the input history path, and confirm line breaks survive round-trip editing.
- Transcript rendering: after submit, confirm chat history shows every prompt line verbatim and does not reinterpret bracket-prefixed lines as attachments.
