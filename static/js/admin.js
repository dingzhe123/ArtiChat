// Admin Panel — article CRUD via fetch API
(function () {
  'use strict';
  var form = document.getElementById('article-form');
  var idField = document.getElementById('article-id');
  var titleField = document.getElementById('title');
  var authorField = document.getElementById('author');
  var tagsField = document.getElementById('tags');
  var tagsInput = document.getElementById('tags-input');
  var tagsChips = document.getElementById('tags-chips');
  var contentField = document.getElementById('content');
  var charCount = document.getElementById('char-count');
  var submitBtn = document.getElementById('submit-btn');
  var cancelBtn = document.getElementById('cancel-btn');
  var formTitle = document.getElementById('form-title');
  var formMsg = document.getElementById('form-msg');
  var tableWrap = document.getElementById('article-table-wrap');
  var editingId = null;
  var chipTags = [];

  // --- Tags chip ---
  function renderChips() {
    tagsChips.innerHTML = '';
    chipTags.forEach(function (t, i) {
      var chip = document.createElement('span');
      chip.className = 'tag-chip';
      chip.innerHTML = t + '<button type="button" class="tag-remove" data-idx="' + i + '">&times;</button>';
      tagsChips.appendChild(chip);
    });
    tagsField.value = chipTags.join(',');
  }

  tagsInput.addEventListener('keydown', function (e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag();
    }
  });
  tagsInput.addEventListener('blur', addTag);

  function addTag() {
    var val = tagsInput.value.trim();
    if (!val) return;
    val.split(',').forEach(function (t) {
      t = t.trim();
      if (t && chipTags.indexOf(t) === -1) {
        chipTags.push(t);
      }
    });
    tagsInput.value = '';
    renderChips();
  }

  tagsChips.addEventListener('click', function (e) {
    if (e.target.classList.contains('tag-remove')) {
      var idx = parseInt(e.target.dataset.idx);
      chipTags.splice(idx, 1);
      renderChips();
    }
  });

  // --- Char count ---
  contentField.addEventListener('input', function () {
    var len = contentField.value.length;
    charCount.textContent = len + ' 字';
  });

  // --- Reset form ---
  function resetForm() {
    editingId = null;
    idField.value = '';
    form.reset();
    chipTags = [];
    renderChips();
    charCount.textContent = '0 字';
    formTitle.textContent = '新建文章';
    submitBtn.textContent = '发布文章';
    cancelBtn.style.display = 'none';
    formMsg.style.display = 'none';
  }

  function showMsg(type, text) {
    formMsg.style.display = 'block';
    formMsg.className = 'form-msg msg-' + type;
    formMsg.textContent = text;
    setTimeout(function () { formMsg.style.display = 'none'; }, 3000);
  }

  // --- Submit ---
  form.addEventListener('submit', async function (e) {
    e.preventDefault();

    var payload = {
      title: titleField.value.trim(),
      author: authorField.value.trim(),
      content: contentField.value.trim(),
      tags: chipTags.slice(),
    };

    if (!payload.title || !payload.content) {
      showMsg('error', '标题和内容不能为空');
      return;
    }

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

  // --- Cancel ---
  cancelBtn.addEventListener('click', resetForm);

  // --- Edit / Delete buttons ---
  tableWrap.addEventListener('click', async function (e) {
    var target = e.target;

    // Edit
    if (target.classList.contains('btn-edit')) {
      var id = target.dataset.id;
      try {
        var res = await fetch('/api/articles/' + id);
        var data = await res.json();
        if (!res.ok || !data.ok) {
          showMsg('error', '加载文章失败');
          return;
        }

        var article = data.data;
        editingId = article.id;
        idField.value = article.id;
        titleField.value = article.title;
        authorField.value = article.author;
        chipTags = (article.tags || []).slice();
        renderChips();
        contentField.value = article.content;
        charCount.textContent = article.content.length + ' 字';
        formTitle.textContent = '编辑文章';
        submitBtn.textContent = '更新文章';
        cancelBtn.style.display = 'inline-block';
        form.scrollIntoView({ behavior: 'smooth' });
      } catch (err) {
        showMsg('error', '加载失败: ' + err.message);
      }
    }

    // Delete
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

        var row = target.closest('tr');
        if (row) row.remove();

        var rows = tableWrap.querySelectorAll('tbody tr');
        if (rows.length === 0) {
          tableWrap.innerHTML = '<div class="empty-state"><p>暂无文章</p><p class="empty-hint">使用左侧表单发布第一篇吧</p></div>';
        }

        var countSpan = document.querySelector('.admin-list .count');
        if (countSpan) countSpan.textContent = '(' + rows.length + ')';

        showMsg('success', '文章已删除');
      } catch (err) {
        showMsg('error', '网络错误: ' + err.message);
      }
    }
  });
})();
