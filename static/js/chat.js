// AI Chat Widget
(function () {
  'use strict';

  var toggle = document.getElementById('chat-toggle');
  var closeBtn = document.getElementById('chat-close');
  var panel = document.getElementById('chat-panel');
  var messagesEl = document.getElementById('chat-messages');
  var loadingEl = document.getElementById('chat-loading');
  var inputEl = document.getElementById('chat-input');
  var sendBtn = document.getElementById('chat-send');

  var isOpen = false;

  // --- Toggle panel ---
  function openPanel() {
    isOpen = true;
    panel.classList.add('open');
    toggle.classList.add('hidden');
    inputEl.focus();
  }

  function closePanel() {
    isOpen = false;
    panel.classList.remove('open');
    toggle.classList.remove('hidden');
  }

  toggle.addEventListener('click', openPanel);
  closeBtn.addEventListener('click', closePanel);

  // --- Escape HTML ---
  function escapeHtml(text) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(text));
    return div.innerHTML;
  }

  // --- Simple markdown-like formatting ---
  function formatText(text) {
    text = escapeHtml(text);
    // Bold
    text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    // Inline code
    text = text.replace(/`([^`]+)`/g, '<code>$1</code>');
    // Newlines
    text = text.replace(/\n/g, '<br>');
    // Headings (## at start of line)
    text = text.replace(/^## (.+)$/gm, '<h4>$1</h4>');
    return text;
  }

  // --- Render message ---
  function addMessage(role, content, sources) {
    var div = document.createElement('div');
    div.className = 'chat-message ' + role;

    var bubble = document.createElement('div');
    bubble.className = 'chat-bubble';
    bubble.innerHTML = formatText(content);

    if (sources && sources.length > 0) {
      var srcDiv = document.createElement('div');
      srcDiv.className = 'chat-sources';
      var srcTitle = document.createElement('div');
      srcTitle.className = 'chat-sources-title';
      srcTitle.textContent = '📚 参考来源';
      srcTitle.addEventListener('click', function () {
        srcDiv.classList.toggle('expanded');
      });
      srcDiv.appendChild(srcTitle);

      var srcList = document.createElement('div');
      srcList.className = 'chat-sources-list';
      sources.forEach(function (s) {
        var item = document.createElement('div');
        item.className = 'chat-source-item';
        item.textContent = s.content;
        srcList.appendChild(item);
      });
      srcDiv.appendChild(srcList);
      bubble.appendChild(srcDiv);
    }

    div.appendChild(bubble);
    messagesEl.appendChild(div);
    scrollToBottom();
  }

  function scrollToBottom() {
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  // --- Loading ---
  function showLoading() {
    loadingEl.classList.add('visible');
    sendBtn.disabled = true;
    inputEl.disabled = true;
  }

  function hideLoading() {
    loadingEl.classList.remove('visible');
    sendBtn.disabled = false;
    inputEl.disabled = false;
    inputEl.focus();
  }

  // --- Send message ---
  function sendMessage() {
    var question = inputEl.value.trim();
    if (!question || sendBtn.disabled) return;

    inputEl.value = '';
    addMessage('user', question);
    showLoading();

    fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question: question }),
    })
      .then(function (res) {
        if (!res.ok) {
          return res.json().then(function (data) {
            throw new Error(data.error || '服务器错误');
          });
        }
        return res.json();
      })
      .then(function (data) {
        hideLoading();
        addMessage('assistant', data.answer, data.sources);
      })
      .catch(function (err) {
        hideLoading();
        addMessage('assistant', '抱歉，出错了 😅\n\n' + escapeHtml(err.message));
      });
  }

  sendBtn.addEventListener('click', sendMessage);
  inputEl.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });
})();
