(function() {
  'use strict';

  if (window.__cpaAIProvidersPluginLoaded) {
    return;
  }
  window.__cpaAIProvidersPluginLoaded = true;

  var cpa = window.CPAExtensions;
  if (!cpa) {
    return;
  }

  function providerDraftKey(provider, index) {
    return provider + ':' + String(index);
  }

  function getProviderDraftLabel(provider, index) {
    if (!cpa.isSupportedProvider(provider) || index == null || index < 0) return null;
    var key = providerDraftKey(provider, index);
    if (!Object.prototype.hasOwnProperty.call(cpa.providerDraftLabels, key)) return null;
    return cpa.trimText(cpa.providerDraftLabels[key]);
  }

  function setProviderDraftLabel(provider, index, label) {
    if (!cpa.isSupportedProvider(provider) || index == null || index < 0) return;
    cpa.providerDraftLabels[providerDraftKey(provider, index)] = cpa.trimText(label);
  }

  function clearProviderDraftLabel(provider, index) {
    if (!cpa.isSupportedProvider(provider) || index == null || index < 0) return;
    delete cpa.providerDraftLabels[providerDraftKey(provider, index)];
  }

  function clearProviderDrafts(provider) {
    if (!provider) {
      cpa.providerDraftLabels = {};
      return;
    }
    var prefix = provider + ':';
    for (var key in cpa.providerDraftLabels) {
      if (Object.prototype.hasOwnProperty.call(cpa.providerDraftLabels, key) && key.indexOf(prefix) === 0) {
        delete cpa.providerDraftLabels[key];
      }
    }
  }

  function removeProviderLabelEditors() {
    document.querySelectorAll('[data-cpa-provider-label-group]').forEach(function(node) {
      if (node && node.parentNode) node.parentNode.removeChild(node);
    });
  }

  function ensureProviderListLabelRow(row, provider, index, label) {
    var meta = row.querySelector('.item-meta') || row;
    var fieldRow = meta.querySelector('[data-cpa-provider-label-row]');
    var valueNode = null;
    if (!fieldRow) {
      var templateRow = row.querySelector('[class*="AiProvidersPage-module__fieldRow___"]');
      var templateLabel = templateRow ? templateRow.querySelector('[class*="AiProvidersPage-module__fieldLabel___"]') : null;
      var templateValue = templateRow ? templateRow.querySelector('[class*="AiProvidersPage-module__fieldValue___"]') : null;
      fieldRow = templateRow ? templateRow.cloneNode(false) : document.createElement('div');
      var labelNode = templateLabel ? templateLabel.cloneNode(false) : document.createElement('span');
      valueNode = templateValue ? templateValue.cloneNode(false) : document.createElement('span');
      fieldRow.innerHTML = '';
      fieldRow.setAttribute('data-cpa-provider-label-row', provider + ':' + index);
      labelNode.textContent = '显示名称';
      valueNode.setAttribute('data-cpa-provider-label-value', '1');
      fieldRow.appendChild(labelNode);
      fieldRow.appendChild(valueNode);
      meta.appendChild(fieldRow);
    } else {
      valueNode = fieldRow.querySelector('[data-cpa-provider-label-value]') || fieldRow.querySelector('[class*="AiProvidersPage-module__fieldValue___"]') || fieldRow.lastElementChild;
    }
    if (!valueNode) return;
    valueNode.textContent = label || '未设置';
    valueNode.style.opacity = label ? '1' : '0.72';
  }

  function renderProviderListSection(provider) {
    var meta = cpa.providerMeta[provider];
    if (!meta) return;
    var section = document.getElementById(meta.sectionId);
    if (!section) return;
    var rows = section.querySelectorAll('.item-row');
    rows.forEach(function(row, index) {
      var entry = cpa.getProviderEntry(provider, index);
      var label = cpa.getEntryLabel(entry);
      var titleNode = row.querySelector('.item-title');
      if (titleNode) {
        if (!titleNode.hasAttribute('data-cpa-original-title')) {
          titleNode.setAttribute('data-cpa-original-title', cpa.trimText(titleNode.textContent) || meta.defaultTitle);
        }
        titleNode.textContent = label || titleNode.getAttribute('data-cpa-original-title') || meta.defaultTitle;
      }
      ensureProviderListLabelRow(row, provider, index, label);
    });
  }

  function findProviderEditCard() {
    return document.querySelector('[class*="AiProvidersEditLayout-module__content___"] .card') ||
      document.querySelector('[class*="SecondaryScreenShell-module__contentWithFloatingAction___"] .card');
  }

  function renderProviderLabelEditor(provider, index) {
    var meta = cpa.providerMeta[provider];
    var entry = cpa.getProviderEntry(provider, index);
    var card = findProviderEditCard();
    if (!meta || !entry || !card) return;

    var group = card.querySelector('[data-cpa-provider-label-group]');
    if (!group) {
      var sampleGroup = card.querySelector('.form-group');
      var sampleInput = card.querySelector('.input');
      group = document.createElement('div');
      group.className = sampleGroup ? sampleGroup.className : 'form-group';
      group.setAttribute('data-cpa-provider-label-group', '1');
      group.innerHTML = '';

      var labelNode = document.createElement('label');
      labelNode.textContent = '自定义名称';
      labelNode.style.display = 'block';
      labelNode.style.marginBottom = '8px';
      labelNode.style.fontWeight = '600';

      var input = document.createElement('input');
      input.type = 'text';
      input.placeholder = '例如：主账号 / 备用线路 / 团队 A';
      input.className = sampleInput ? sampleInput.className : 'input';
      input.setAttribute('data-cpa-provider-label-input', '1');

      var hint = document.createElement('div');
      hint.textContent = '仅修改当前条目的显示名称，不会改动 provider 分组标题。';
      hint.style.fontSize = '12px';
      hint.style.opacity = '0.72';
      hint.style.marginTop = '8px';

      var actions = document.createElement('div');
      actions.style.display = 'flex';
      actions.style.alignItems = 'center';
      actions.style.gap = '10px';
      actions.style.marginTop = '12px';

      var saveBtn = document.createElement('button');
      saveBtn.type = 'button';
      saveBtn.textContent = '保存名称';
      saveBtn.setAttribute('data-cpa-provider-label-save', '1');
      saveBtn.style.border = 'none';
      saveBtn.style.borderRadius = '8px';
      saveBtn.style.padding = '8px 14px';
      saveBtn.style.cursor = 'pointer';
      saveBtn.style.fontWeight = '600';
      saveBtn.style.background = '#89b4fa';
      saveBtn.style.color = '#11111b';

      var status = document.createElement('span');
      status.setAttribute('data-cpa-provider-label-status', '1');
      status.style.fontSize = '12px';
      status.style.minHeight = '18px';
      status.style.color = '#a6adc8';

      actions.appendChild(saveBtn);
      actions.appendChild(status);
      group.appendChild(labelNode);
      group.appendChild(input);
      group.appendChild(hint);
      group.appendChild(actions);
      card.appendChild(group);

      input.addEventListener('input', function() {
        var current = cpa.trimText(input.value);
        var synced = cpa.trimText(input.getAttribute('data-cpa-synced-label'));
        var currentProvider = cpa.trimText(group.getAttribute('data-cpa-provider-name'));
        var indexText = group.getAttribute('data-cpa-provider-index');
        var currentIndex = /^-?\d+$/.test(indexText || '') ? parseInt(indexText, 10) : null;
        if (currentProvider && currentIndex != null && currentIndex >= 0) {
          if (current === synced) {
            clearProviderDraftLabel(currentProvider, currentIndex);
          } else {
            setProviderDraftLabel(currentProvider, currentIndex, current);
          }
        }
        status.textContent = current === synced ? '' : '有未保存更改';
        status.style.color = current === synced ? '#a6adc8' : '#f9e2af';
      });
      input.addEventListener('keydown', function(evt) {
        if (evt.key === 'Enter') {
          evt.preventDefault();
          saveBtn.click();
        }
      });
    }

    var inputNode = group.querySelector('[data-cpa-provider-label-input]');
    var saveNode = group.querySelector('[data-cpa-provider-label-save]');
    var statusNode = group.querySelector('[data-cpa-provider-label-status]');
    if (!inputNode || !saveNode || !statusNode) return;

    var syncedLabel = cpa.getEntryLabel(entry);
    var draftLabel = getProviderDraftLabel(provider, index);
    var nextLabel = draftLabel !== null ? draftLabel : syncedLabel;
    var currentValue = cpa.trimText(inputNode.value);
    var syncedValue = cpa.trimText(inputNode.getAttribute('data-cpa-synced-label'));
    group.setAttribute('data-cpa-provider-name', provider);
    group.setAttribute('data-cpa-provider-index', String(index));
    if (document.activeElement !== inputNode || currentValue === syncedValue) {
      inputNode.value = nextLabel;
    }
    inputNode.setAttribute('data-cpa-synced-label', syncedLabel);

    if (cpa.trimText(inputNode.value) === syncedLabel && statusNode.textContent === '有未保存更改') {
      statusNode.textContent = '';
      statusNode.style.color = '#a6adc8';
    } else if (cpa.trimText(inputNode.value) !== syncedLabel) {
      statusNode.textContent = '有未保存更改';
      statusNode.style.color = '#f9e2af';
    }

    saveNode.onclick = function() {
      var labelValue = cpa.trimText(inputNode.value);
      saveNode.disabled = true;
      saveNode.style.opacity = '0.7';
      statusNode.textContent = '保存中...';
      statusNode.style.color = '#89b4fa';
      cpa.doFetch(cpa.base + meta.endpoint, {
        method: 'PATCH',
        headers: cpa.mgmtHeaders(),
        body: JSON.stringify({index: index, value: {label: labelValue}})
      }).then(cpa.readJSON)
        .then(function() {
          return cpa.fetchProviderEntries(provider, true);
        })
        .then(function() {
          var savedEntry = cpa.getProviderEntry(provider, index);
          var savedLabel = cpa.getEntryLabel(savedEntry || {label: labelValue});
          clearProviderDraftLabel(provider, index);
          inputNode.value = savedLabel;
          inputNode.setAttribute('data-cpa-synced-label', savedLabel);
          statusNode.textContent = savedLabel ? '显示名称已保存' : '已恢复默认名称';
          statusNode.style.color = '#a6e3a1';
          cpa.queueAIProvidersSync(50);
        }, function(err) {
          statusNode.textContent = '保存失败：' + ((err && err.message) ? err.message : 'unknown error');
          statusNode.style.color = '#f38ba8';
        })
        .then(function() {
          saveNode.disabled = false;
          saveNode.style.opacity = '1';
        });
    };
  }

  function trySyncAIProviders() {
    var info = cpa.parseAIProvidersHash();
    if (!info) {
      removeProviderLabelEditors();
      return;
    }

    if (info.isList) {
      removeProviderLabelEditors();
      Promise.all(Object.keys(cpa.providerMeta).map(function(provider) {
        return cpa.fetchProviderEntries(provider, false).catch(function() { return []; });
      })).then(function() {
        Object.keys(cpa.providerMeta).forEach(renderProviderListSection);
      }).catch(function() {});
      return;
    }

    if (!cpa.isSupportedProvider(info.provider) || info.index == null || info.index < 0) {
      removeProviderLabelEditors();
      return;
    }

    cpa.fetchProviderEntries(info.provider, false).then(function() {
      renderProviderLabelEditor(info.provider, info.index);
      renderProviderListSection(info.provider);
    }).catch(function() {});
  }

  function scheduleAIProvidersSync(delay) {
    if (cpa.aiSyncTimer) return;
    cpa.aiSyncTimer = setTimeout(function() {
      cpa.aiSyncTimer = 0;
      trySyncAIProviders();
    }, delay || 100);
  }

  function currentEditLabelFor(provider, index) {
    var info = cpa.parseAIProvidersHash();
    if (!info || info.provider !== provider || info.index !== index) return null;
    var input = document.querySelector('[data-cpa-provider-label-input]');
    if (!input) return null;
    return cpa.trimText(input.value);
  }

  function mergeLabelsIntoProviderPayload(provider, payload) {
    var entries = cpa.providerCache[provider] || [];

    function mergeItem(item, index) {
      if (!item || typeof item !== 'object' || Array.isArray(item)) return item;
      var copy = {};
      for (var key in item) {
        if (Object.prototype.hasOwnProperty.call(item, key)) copy[key] = item[key];
      }
      var draftLabel = getProviderDraftLabel(provider, index);
      var liveLabel = currentEditLabelFor(provider, index);
      var effectiveLabel = draftLabel !== null ? draftLabel : liveLabel;
      var cachedEntry = entries[index];
      var hasCachedLabel = !!(cachedEntry && Object.prototype.hasOwnProperty.call(cachedEntry, 'label'));
      if (effectiveLabel !== null || hasCachedLabel) {
        copy.label = effectiveLabel !== null ? effectiveLabel : cpa.getEntryLabel(cachedEntry);
      }
      return copy;
    }

    if (Array.isArray(payload)) {
      return payload.map(mergeItem);
    }
    if (payload && Array.isArray(payload.items)) {
      payload.items = payload.items.map(mergeItem);
    }
    return payload;
  }

  cpa.getProviderDraftLabel = getProviderDraftLabel;
  cpa.setProviderDraftLabel = setProviderDraftLabel;
  cpa.clearProviderDraftLabel = clearProviderDraftLabel;
  cpa.clearProviderDrafts = clearProviderDrafts;
  cpa.scheduleAIProvidersSync = scheduleAIProvidersSync;
  cpa.prepareProviderRequest = function(provider, method, input, init) {
    if (method !== 'PUT' || !init || typeof init.body !== 'string') {
      return init;
    }
    try {
      var parsedBody = JSON.parse(init.body);
      parsedBody = mergeLabelsIntoProviderPayload(provider, parsedBody);
      var nextInit = {};
      for (var key in init) {
        if (Object.prototype.hasOwnProperty.call(init, key)) nextInit[key] = init[key];
      }
      nextInit.body = JSON.stringify(parsedBody);
      return nextInit;
    } catch (e) {
      return init;
    }
  };
  cpa.afterProviderRequest = function(provider, method, resp) {
    if (!resp || !resp.ok) return;
    if (method === 'PUT' || method === 'DELETE') {
      clearProviderDrafts(provider);
    }
    cpa.fetchProviderEntries(provider, true).then(function() {
      cpa.queueAIProvidersSync(80);
    }).catch(function() {});
  };

  setTimeout(function() {
    if (location.hash.indexOf('#/ai-providers') === 0) {
      cpa.queueAIProvidersSync(120);
    }
  }, 50);
})();
