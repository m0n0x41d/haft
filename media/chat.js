// @ts-check
(function () {
  // @ts-ignore
  const vscode = acquireVsCodeApi();

  const messagesEl = document.getElementById('messages');
  const inputEl = /** @type {HTMLTextAreaElement} */ (document.getElementById('input'));
  const sendBtn = document.getElementById('send-btn');
  const abortBtn = document.getElementById('abort-btn');
  const providerBtn = document.getElementById('provider-btn');
  const clearBtn = document.getElementById('clear-btn');
  const providerName = document.getElementById('provider-name');
  const mcpStatus = document.getElementById('mcp-status');
  const autocompleteEl = document.getElementById('autocomplete');

  let isStreaming = false;
  let streamingEl = null;
  let acIndex = -1;

  // --- Message rendering ---

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function renderMarkdown(text) {
    let html = escapeHtml(text);

    // Code blocks
    html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>');

    // Inline code
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

    // Bold
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');

    // Italic
    html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');

    // Line breaks
    html = html.replace(/\n/g, '<br>');

    return html;
  }

  function addMessage(type, text) {
    removeThinking();

    const div = document.createElement('div');
    div.className = `message ${type}`;

    if (type === 'user') {
      div.textContent = text;
    } else if (type === 'thinking') {
      div.textContent = text;
    } else {
      div.innerHTML = renderMarkdown(text);
    }

    messagesEl.appendChild(div);
    scrollToBottom();
    return div;
  }

  function removeThinking() {
    const thinking = messagesEl.querySelectorAll('.message.thinking');
    thinking.forEach((el) => el.remove());
  }

  function scrollToBottom() {
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  // --- Streaming ---

  function startStream() {
    removeThinking();
    streamingEl = document.createElement('div');
    streamingEl.className = 'message assistant';
    messagesEl.appendChild(streamingEl);
    isStreaming = true;
    sendBtn.classList.add('hidden');
    abortBtn.classList.remove('hidden');
  }

  function appendStream(text) {
    if (!streamingEl) {
      startStream();
    }
    streamingEl._rawText = (streamingEl._rawText || '') + text;
    streamingEl.innerHTML = renderMarkdown(streamingEl._rawText);
    scrollToBottom();
  }

  function endStream() {
    streamingEl = null;
    isStreaming = false;
    sendBtn.classList.remove('hidden');
    abortBtn.classList.add('hidden');
  }

  // --- Autocomplete ---

  function showAutocomplete(filter) {
    const matches = COMMANDS.filter(
      (c) => c.name.startsWith(filter) && c.name !== filter,
    );

    if (matches.length === 0) {
      hideAutocomplete();
      return;
    }

    autocompleteEl.innerHTML = '';
    acIndex = -1;

    matches.forEach((cmd, i) => {
      const item = document.createElement('div');
      item.className = 'ac-item';
      item.innerHTML = `<span class="cmd">${escapeHtml(cmd.name)}</span><span class="desc">${escapeHtml(cmd.description)}</span>`;
      item.addEventListener('click', () => {
        inputEl.value = cmd.name + ' ';
        hideAutocomplete();
        inputEl.focus();
      });
      autocompleteEl.appendChild(item);
    });

    autocompleteEl.classList.remove('hidden');
  }

  function hideAutocomplete() {
    autocompleteEl.classList.add('hidden');
    autocompleteEl.innerHTML = '';
    acIndex = -1;
  }

  function selectAcItem(direction) {
    const items = autocompleteEl.querySelectorAll('.ac-item');
    if (items.length === 0) return;

    if (acIndex >= 0) {
      items[acIndex].classList.remove('selected');
    }

    acIndex += direction;
    if (acIndex < 0) acIndex = items.length - 1;
    if (acIndex >= items.length) acIndex = 0;

    items[acIndex].classList.add('selected');
  }

  function applyAcSelection() {
    const items = autocompleteEl.querySelectorAll('.ac-item');
    if (acIndex >= 0 && acIndex < items.length) {
      const cmdText = items[acIndex].querySelector('.cmd').textContent;
      inputEl.value = cmdText + ' ';
      hideAutocomplete();
      return true;
    }
    return false;
  }

  // --- Input handling ---

  function send() {
    const text = inputEl.value.trim();
    if (!text) return;

    inputEl.value = '';
    inputEl.style.height = 'auto';
    hideAutocomplete();

    vscode.postMessage({ type: 'send', text });
  }

  inputEl.addEventListener('input', () => {
    // Auto-resize
    inputEl.style.height = 'auto';
    inputEl.style.height = Math.min(inputEl.scrollHeight, 120) + 'px';

    // Autocomplete
    const val = inputEl.value;
    if (val.startsWith('/') && !val.includes(' ')) {
      showAutocomplete(val);
    } else {
      hideAutocomplete();
    }
  });

  inputEl.addEventListener('keydown', (e) => {
    if (!autocompleteEl.classList.contains('hidden')) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectAcItem(1);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectAcItem(-1);
        return;
      }
      if (e.key === 'Tab' || (e.key === 'Enter' && acIndex >= 0)) {
        e.preventDefault();
        applyAcSelection();
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        hideAutocomplete();
        return;
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  });

  sendBtn.addEventListener('click', send);
  abortBtn.addEventListener('click', () => {
    vscode.postMessage({ type: 'abort' });
  });

  providerBtn.addEventListener('click', () => {
    vscode.postMessage({ type: 'selectProvider' });
  });

  clearBtn.addEventListener('click', () => {
    messagesEl.innerHTML = '';
    vscode.postMessage({ type: 'clear' });
  });

  // --- Messages from extension ---

  window.addEventListener('message', (event) => {
    const msg = event.data;

    switch (msg.type) {
      case 'user':
        addMessage('user', msg.text);
        break;

      case 'assistant':
        endStream();
        addMessage('assistant', msg.text);
        break;

      case 'thinking':
        removeThinking();
        addMessage('thinking', msg.text);
        break;

      case 'doneThinking':
        removeThinking();
        break;

      case 'stream':
        appendStream(msg.text);
        break;

      case 'streamEnd':
        endStream();
        break;

      case 'toolCall':
        addMessage('tool-call', `Tool: ${msg.name}\nArgs: ${JSON.stringify(msg.args, null, 2)}`);
        break;

      case 'toolResult': {
        const el = addMessage('tool-result', `${msg.name}: ${msg.result}`);
        if (msg.isError) {
          el.classList.add('error');
        }
        break;
      }

      case 'providerInfo':
        providerName.textContent = msg.provider;
        if (msg.mcpConnected) {
          mcpStatus.classList.add('connected');
          mcpStatus.title = 'MCP server connected';
        } else {
          mcpStatus.classList.remove('connected');
          mcpStatus.title = 'MCP server disconnected';
        }
        break;

      case 'clear':
        messagesEl.innerHTML = '';
        break;

      case 'error':
        addMessage('assistant', `**Error:** ${msg.text}`);
        break;
    }
  });

  // Signal ready
  vscode.postMessage({ type: 'ready' });
})();
