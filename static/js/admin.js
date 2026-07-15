// Admin Panel — article CRUD via fetch API
(function () {
  var form = document.getElementById('article-form');
  var idField = document.getElementById('article-id');
  var titleField = document.getElementById('title');
  var authorField = document.getElementById('author');
  var tagsField = document.getElementById('tags');
  var contentField = document.getElementById('content');
  var submitBtn = document.getElementById('submit-btn');
  var cancelBtn = document.getElementById('cancel-btn');
  var formTitle = document.getElementById('form-title');
  var formMsg = document.getElementById('form-msg');
  var tableWrap = document.getElementById('article-table-wrap');
  var editingId = null;

  // Reset form to "create" mode
  function resetForm() {
    editingId = null;
    idField.value = '';
    form.reset();
    formTitle.textContent = '新建文章';
    submitBtn.textContent = '发布文章';
    cancelBtn.style.display = 'none';
    formMsg.style.display = 'none';
  }

  // Show a message
  function showMsg(type, text) {
    formMsg.style.display = 'block';
    formMsg.className = 'form-msg msg-' + type;
    formMsg.textContent = text;
    setTimeout(function () {
      formMsg.style.display = 'none';
    }, 3000);
  }

  // Submit: create or update
  form.addEventListener('submit', async function (e) {
    e.preventDefault();

    var tags = tagsField.value
      .split(',')
      .map(function (t) { return t.trim(); })
      .filter(function (t) { return t !== ''; });

    var payload = {
      title: titleField.value.trim(),
      author: authorField.value.trim(),
      content: contentField.value.trim(),
      tags: tags,
    };

    var isEdit = editingId !== null;
    var url = isEdit ? '/api/articles/' + editingId : '/api/articles';
    var method = isEdit ? 'PUT' : 'POST';

    try {
      var res = await fetch(url, {
        method: method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      var data = await res.json();

      if (!res.ok || !data.ok) {
        showMsg('error', '操作失败: ' + (data.error || res.statusText));
        return;
      }

      showMsg('success', isEdit ? '文章已更新' : '文章已发布');
      resetForm();
      location.reload();
    } catch (err) {
      showMsg('error', '网络错误: ' + err.message);
    }
  });

  // Cancel editing
  cancelBtn.addEventListener('click', resetForm);

  // Delegate clicks for edit / delete buttons
  tableWrap.addEventListener('click', async function (e) {
    var target = e.target;

    // --- Edit ---
    if (target.classList.contains('btn-edit')) {
      var id = target.dataset.id;
      try {
        var res = await fetch('/api/articles/' + id);
        var data = await res.json();

        if (!res.ok || !data.ok) {
          showMsg('error', '加载文章失败: ' + (data.error || res.statusText));
          return;
        }

        var article = data.data;

        // Populate form
        editingId = article.id;
        idField.value = article.id;
        titleField.value = article.title;
        authorField.value = article.author;
        tagsField.value = (article.tags || []).join(', ');
        contentField.value = article.content;
        formTitle.textContent = '编辑文章';
        submitBtn.textContent = '更新文章';
        cancelBtn.style.display = 'inline-block';

        // Scroll to form
        form.scrollIntoView({ behavior: 'smooth' });
      } catch (err) {
        showMsg('error', '加载文章失败: ' + err.message);
      }
    }

    // --- Delete ---
    if (target.classList.contains('btn-delete')) {
      var id = target.dataset.id;
      if (!confirm('确定要删除这篇文章吗？此操作不可撤销。')) return;

      try {
        var res = await fetch('/api/articles/' + id, { method: 'DELETE' });
        var data = await res.json();

        if (!res.ok || !data.ok) {
          showMsg('error', '删除失败: ' + (data.error || res.statusText));
          return;
        }

        // Remove the row
        var row = target.closest('tr');
        if (row) row.remove();

        // Check if table is empty
        var rows = tableWrap.querySelectorAll('tbody tr');
        if (rows.length === 0) {
          tableWrap.innerHTML = '<p class="empty-msg">暂无文章，使用左侧表单发布第一篇吧 🚀</p>';
        }

        // Update count
        var countSpan = document.querySelector('.admin-list .count');
        if (countSpan) {
          countSpan.textContent = '(' + rows.length + ')';
        }

        showMsg('success', '文章已删除');
      } catch (err) {
        showMsg('error', '网络错误: ' + err.message);
      }
    }
  });
})();
