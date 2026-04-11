(function() {
  'use strict';

  if (window.__cpaUsagePluginLoaded) {
    return;
  }
  window.__cpaUsagePluginLoaded = true;

  var cpa = window.CPAExtensions;
  if (!cpa) {
    return;
  }

  var detailCache = {};
  var detailCacheLoaded = false;

  function ensureUsageDOM() {
    if (!document.body) return false;

    if (!document.getElementById('cpa-fab')) {
      var fab = document.createElement('button');
      fab.id = 'cpa-fab';
      fab.title = 'SQLite 数据保留设置';
      fab.textContent = '⚙';
      document.body.appendChild(fab);
      fab.onclick = function() {
        var panel = document.getElementById('cpa-ret-panel');
        if (!panel) return;
        var open = panel.classList.toggle('open');
        if (open) loadRetention();
      };
    }

    if (!document.getElementById('cpa-ret-panel')) {
      var panel = document.createElement('div');
      panel.id = 'cpa-ret-panel';
      panel.innerHTML =
        '<h4>📦 SQLite 数据保留</h4>' +
        '<div class="cpa-row">' +
        '  <span style="color:#a6adc8">保留天数</span>' +
        '  <input type="number" id="cpa-days" min="1" max="3650" value="90">' +
        '  <button class="save-btn" id="cpa-save">保存</button>' +
        '</div>' +
        '<div id="cpa-ret-msg"></div>';
      document.body.appendChild(panel);
    }

    if (!document.getElementById('cpa-detail-mask')) {
      var mask = document.createElement('div');
      mask.id = 'cpa-detail-mask';
      mask.innerHTML =
        '<div id="cpa-detail-card">' +
        '  <h3>📋 请求详情</h3>' +
        '  <div id="cpa-detail-body"></div>' +
        '  <button class="close-btn" id="cpa-detail-close">关闭</button>' +
        '</div>';
      document.body.appendChild(mask);
      mask.addEventListener('click', function(e) {
        if (e.target === mask) mask.classList.remove('open');
      });
    }

    var closeBtn = document.getElementById('cpa-detail-close');
    if (closeBtn && !closeBtn.__cpaBound) {
      closeBtn.__cpaBound = true;
      closeBtn.onclick = function() {
        var mask = document.getElementById('cpa-detail-mask');
        if (mask) mask.classList.remove('open');
      };
    }

    var saveBtn = document.getElementById('cpa-save');
    if (saveBtn && !saveBtn.__cpaBound) {
      saveBtn.__cpaBound = true;
      saveBtn.onclick = function() {
        var input = document.getElementById('cpa-days');
        var days = parseInt(input && input.value, 10);
        if (!days || days < 1 || days > 3650) {
          alert('请输入 1–3650 之间的天数');
          return;
        }
        cpa.doFetch(cpa.base + '/v0/management/usage/retention', {
          method: 'PUT',
          headers: cpa.mgmtHeaders(),
          body: JSON.stringify({retention_days: days})
        }).then(cpa.readJSON)
          .then(function(payload) {
            var msg = document.getElementById('cpa-ret-msg');
            if (!msg) return;
            msg.textContent = '✅ 已保存：' + (payload.retention_days || days) + ' 天';
            setTimeout(function() {
              msg.textContent = '';
            }, 3000);
          }).catch(function(err) {
            alert('保存失败：' + ((err && err.message) ? err.message : 'unknown error'));
          });
      };
    }

    if (!document.__cpaUsageOutsideClickBound) {
      document.__cpaUsageOutsideClickBound = true;
      document.addEventListener('click', function(e) {
        var panel = document.getElementById('cpa-ret-panel');
        var fab = document.getElementById('cpa-fab');
        if (!panel || !fab) return;
        if (!panel.contains(e.target) && e.target !== fab) {
          panel.classList.remove('open');
        }
      });
    }

    return true;
  }

  function loadRetention() {
    cpa.doFetch(cpa.base + '/v0/management/usage/retention', {headers: cpa.mgmtHeaders()})
      .then(cpa.readJSON)
      .then(function(payload) {
        var input = document.getElementById('cpa-days');
        if (input) input.value = payload.retention_days || 90;
      }).catch(function() {});
  }

  function loadDetailCache(cb) {
    if (detailCacheLoaded) {
      if (cb) cb();
      return;
    }
    cpa.doFetch(cpa.base + '/v0/management/usage/details?limit=5000', {headers: cpa.mgmtHeaders()})
      .then(function(resp) { return resp.json(); })
      .then(function(payload) {
        detailCacheLoaded = true;
        (payload.details || []).forEach(function(item) {
          var key = (item.timestamp || '') + '|' + (item.model || '');
          detailCache[key] = item;
        });
        if (cb) cb();
      }).catch(function() {});
  }

  function showDetail(item) {
    function esc(value) {
      return String(value == null ? '-' : value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
    }

    ensureUsageDOM();
    var body = document.getElementById('cpa-detail-body');
    var mask = document.getElementById('cpa-detail-mask');
    if (!body || !mask) return;

    var tok = item.tokens || {};
    var rows = [
      ['时间', item.timestamp ? new Date(item.timestamp).toLocaleString() : '-'],
      ['模型', item.model || '-'],
      ['API / Provider', item.api || '-'],
      ['来源 (source)', item.source || '-'],
      ['认证索引', item.auth_index || '-'],
      ['延迟', (item.latency_ms || 0) + ' ms'],
      ['状态', item.failed ? '❌ 失败' : '✅ 成功'],
      ['输入 Token', tok.input_tokens || 0],
      ['输出 Token', tok.output_tokens || 0],
      ['思考 Token', tok.reasoning_tokens || 0],
      ['缓存 Token', tok.cached_tokens || 0],
      ['总 Token', tok.total_tokens || 0]
    ];

    body.innerHTML = rows.map(function(row) {
      return '<div class="drow"><span class="dlabel">' + esc(row[0]) + '</span><span class="dval">' + esc(row[1]) + '</span></div>';
    }).join('');
    mask.classList.add('open');
  }

  function hookRow(tr) {
    if (tr.dataset.cpaHooked) return;
    tr.dataset.cpaHooked = '1';
    tr.classList.add('cpa-clickable');
    tr.addEventListener('click', function(e) {
      if (e.target.tagName === 'BUTTON' || e.target.tagName === 'A') return;
      var cells = tr.querySelectorAll('td');
      if (cells.length < 2) return;
      var tsText = cells[0] ? cpa.trimText(cells[0].textContent) : '';
      var modelText = cells[1] ? cpa.trimText(cells[1].textContent) : '';
      loadDetailCache(function() {
        var found = null;
        Object.keys(detailCache).forEach(function(key) {
          var item = detailCache[key];
          if (!found && item.model && modelText && item.model.indexOf(modelText) >= 0) {
            if (item.timestamp) {
              var itemTs = new Date(item.timestamp).toLocaleString();
              if (tsText && itemTs.indexOf(tsText.slice(0, 10)) >= 0) {
                found = item;
              }
            }
            if (!found) found = item;
          }
        });
        if (found) {
          showDetail(found);
          return;
        }
        showDetail({
          timestamp: tsText,
          model: modelText,
          source: cells[2] ? cpa.trimText(cells[2].textContent) : '-',
          auth_index: cells[3] ? cpa.trimText(cells[3].textContent) : '-',
          tokens: {
            input_tokens: cells[5] ? parseInt(cells[5].textContent, 10) || 0 : 0,
            output_tokens: cells[6] ? parseInt(cells[6].textContent, 10) || 0 : 0,
            reasoning_tokens: cells[7] ? parseInt(cells[7].textContent, 10) || 0 : 0,
            cached_tokens: cells[8] ? parseInt(cells[8].textContent, 10) || 0 : 0,
            total_tokens: cells[9] ? parseInt(cells[9].textContent, 10) || 0 : 0
          }
        });
      });
    });
  }

  function hookTableRows() {
    var tables = document.querySelectorAll('table');
    tables.forEach(function(tbl) {
      if (tbl.dataset.cpaHooked) return;
      var firstRow = tbl.querySelector('tbody tr');
      if (!firstRow) return;
      tbl.dataset.cpaHooked = '1';
      tbl.querySelectorAll('tbody tr').forEach(hookRow);
      var obs = new MutationObserver(function(mutations) {
        mutations.forEach(function(mutation) {
          mutation.addedNodes.forEach(function(node) {
            if (node.tagName === 'TR') hookRow(node);
          });
        });
      });
      obs.observe(tbl.querySelector('tbody') || tbl, {childList: true, subtree: true});
    });
  }

  if (!ensureUsageDOM()) {
    return;
  }

  var pageObs = new MutationObserver(function() {
    if (location.hash.includes('usage')) hookTableRows();
    if (location.hash.indexOf('#/ai-providers') === 0) cpa.queueAIProvidersSync(80);
  });
  pageObs.observe(document.body, {childList: true, subtree: true});

  window.addEventListener('hashchange', function() {
    detailCacheLoaded = false;
    detailCache = {};
    if (location.hash.includes('usage')) setTimeout(hookTableRows, 600);
    if (location.hash.indexOf('#/ai-providers') === 0) {
      setTimeout(function() { cpa.queueAIProvidersSync(80); }, 120);
    }
  });

  setTimeout(function() {
    if (location.hash.includes('usage')) hookTableRows();
    if (location.hash.indexOf('#/ai-providers') === 0) cpa.queueAIProvidersSync(120);
  }, 1000);
})();
