(function() {
  'use strict';

  if (window.__cpaBootstrapLoaded) {
    return;
  }
  window.__cpaBootstrapLoaded = true;

  var currentScript = document.currentScript;
  var assetBase = '/-/management-assets';
  var assetVersion = '';
  try {
    if (currentScript && currentScript.src) {
      var parsed = new URL(currentScript.src, location.href);
      assetBase = parsed.pathname.replace(/\/bootstrap\.js$/, '') || assetBase;
      assetVersion = String(parsed.searchParams.get('v') || '').replace(/^\s+|\s+$/g, '');
    }
  } catch (e) {}

  function trimText(value) {
    return String(value == null ? '' : value).replace(/^\s+|\s+$/g, '');
  }

  function assetURL(name) {
    var cleaned = trimText(name).replace(/^\/+/, '');
    if (!cleaned) return '';
    var url = assetBase + '/' + cleaned;
    if (assetVersion) {
      url += '?v=' + encodeURIComponent(assetVersion);
    }
    return url;
  }

  var nativeFetch = typeof window.fetch === 'function' ? window.fetch.bind(window) : null;
  var runtime = window.CPAExtensions || {};
  runtime.base = location.protocol + '//' + location.host;
  runtime.assetBase = assetBase;
  runtime.assetVersion = assetVersion;
  runtime.assetURL = assetURL;
  runtime.nativeFetch = runtime.nativeFetch || nativeFetch;
  runtime.cachedMgmtAuthorization = runtime.cachedMgmtAuthorization || '';
  runtime.cachedMgmtXKey = runtime.cachedMgmtXKey || '';
  runtime.providerMeta = runtime.providerMeta || {
    gemini: {endpoint: '/v0/management/gemini-api-key', responseKey: 'gemini-api-key', sectionId: 'provider-gemini', defaultTitle: 'Gemini密钥'},
    codex: {endpoint: '/v0/management/codex-api-key', responseKey: 'codex-api-key', sectionId: 'provider-codex', defaultTitle: 'Codex配置'},
    claude: {endpoint: '/v0/management/claude-api-key', responseKey: 'claude-api-key', sectionId: 'provider-claude', defaultTitle: 'Claude配置'},
    vertex: {endpoint: '/v0/management/vertex-api-key', responseKey: 'vertex-api-key', sectionId: 'provider-vertex', defaultTitle: 'Vertex配置'}
  };
  runtime.providerCache = runtime.providerCache || {};
  runtime.providerInflight = runtime.providerInflight || {};
  runtime.providerDraftLabels = runtime.providerDraftLabels || {};
  runtime.aiSyncTimer = runtime.aiSyncTimer || 0;
  runtime.pendingScripts = runtime.pendingScripts || {};
  runtime.trimText = trimText;
  runtime.getEntryLabel = function(entry) {
    return trimText(entry && entry.label);
  };
  runtime.readHeaderValue = function(headers, name) {
    if (!headers || !name) return '';
    var lowerName = String(name).toLowerCase();
    if (typeof headers.get === 'function') {
      return trimText(headers.get(name) || headers.get(lowerName) || '');
    }
    if (Array.isArray(headers)) {
      for (var i = 0; i < headers.length; i++) {
        var pair = headers[i];
        if (Array.isArray(pair) && pair.length >= 2 && String(pair[0]).toLowerCase() === lowerName) {
          return trimText(pair[1]);
        }
      }
      return '';
    }
    if (typeof headers === 'object') {
      for (var key in headers) {
        if (Object.prototype.hasOwnProperty.call(headers, key) && String(key).toLowerCase() === lowerName) {
          return trimText(headers[key]);
        }
      }
    }
    return '';
  };
  runtime.rememberMgmtHeaders = function(headers) {
    if (!headers) return false;
    var changed = false;
    var authHeader = runtime.readHeaderValue(headers, 'Authorization');
    var xKeyHeader = runtime.readHeaderValue(headers, 'X-Management-Key');
    if (authHeader && authHeader !== runtime.cachedMgmtAuthorization) {
      runtime.cachedMgmtAuthorization = authHeader;
      changed = true;
    }
    if (xKeyHeader && xKeyHeader !== runtime.cachedMgmtXKey) {
      runtime.cachedMgmtXKey = xKeyHeader;
      changed = true;
    }
    return changed;
  };
  runtime.hasMgmtCredential = function() {
    var key = '';
    try {
      key = localStorage.getItem('managementKey') || sessionStorage.getItem('managementKey') || '';
    } catch (e) {}
    key = trimText(key);
    return !!(key || runtime.cachedMgmtXKey || runtime.cachedMgmtAuthorization);
  };
  runtime.mgmtHeaders = function() {
    var headers = {'Content-Type': 'application/json'};
    var key = '';
    try {
      key = localStorage.getItem('managementKey') || sessionStorage.getItem('managementKey') || '';
    } catch (e) {}
    key = trimText(key);
    if (key) {
      headers['X-Management-Key'] = key;
      return headers;
    }
    if (runtime.cachedMgmtXKey) {
      headers['X-Management-Key'] = runtime.cachedMgmtXKey;
      return headers;
    }
    if (runtime.cachedMgmtAuthorization) {
      headers['Authorization'] = runtime.cachedMgmtAuthorization;
    }
    return headers;
  };
  runtime.doFetch = function(url, options) {
    if (!runtime.nativeFetch) {
      return Promise.reject(new Error('fetch unavailable'));
    }
    return runtime.nativeFetch(url, options || {});
  };
  runtime.readJSON = function(resp) {
    if (!resp.ok) {
      return resp.text().then(function(text) {
        throw new Error(text || ('HTTP ' + resp.status));
      });
    }
    return resp.json();
  };
  runtime.parseAIProvidersHash = function() {
    var matched = location.hash.match(/^#\/ai-providers(?:\/([^\/?#]+)(?:\/([^\/?#]+))?)?/);
    if (!matched) return null;
    var provider = matched[1] ? matched[1].toLowerCase() : '';
    var index = null;
    if (matched[2] && /^-?\d+$/.test(matched[2])) {
      index = parseInt(matched[2], 10);
    }
    return {provider: provider, index: index, isList: provider === ''};
  };
  runtime.isSupportedProvider = function(provider) {
    return !!runtime.providerMeta[provider];
  };
  runtime.matchProviderRequest = function(url) {
    try {
      var parsed = new URL(url, runtime.base);
      var pathname = parsed.pathname;
      for (var key in runtime.providerMeta) {
        if (Object.prototype.hasOwnProperty.call(runtime.providerMeta, key) && runtime.providerMeta[key].endpoint === pathname) {
          return key;
        }
      }
    } catch (e) {}
    return '';
  };
  runtime.isManagementRequest = function(url) {
    try {
      return new URL(url, runtime.base).pathname.indexOf('/v0/management/') === 0;
    } catch (e) {}
    return false;
  };
  runtime.syncProviderCache = function(provider, payload) {
    var meta = runtime.providerMeta[provider];
    if (!meta) return [];
    var entries = payload && Array.isArray(payload[meta.responseKey]) ? payload[meta.responseKey] : [];
    runtime.providerCache[provider] = entries;
    return entries;
  };
  runtime.fetchProviderEntries = function(provider, force) {
    if (!runtime.isSupportedProvider(provider)) return Promise.resolve([]);
    if (!force && runtime.providerCache[provider]) return Promise.resolve(runtime.providerCache[provider]);
    if (!runtime.hasMgmtCredential()) return Promise.resolve(runtime.providerCache[provider] || []);
    if (!force && runtime.providerInflight[provider]) return runtime.providerInflight[provider];

    runtime.providerInflight[provider] = runtime.doFetch(runtime.base + runtime.providerMeta[provider].endpoint, {headers: runtime.mgmtHeaders()})
      .then(runtime.readJSON)
      .then(function(payload) {
        delete runtime.providerInflight[provider];
        return runtime.syncProviderCache(provider, payload);
      }, function(err) {
        delete runtime.providerInflight[provider];
        throw err;
      });

    return runtime.providerInflight[provider];
  };
  runtime.getProviderEntry = function(provider, index) {
    var entries = runtime.providerCache[provider] || [];
    if (index == null || index < 0 || index >= entries.length) return null;
    return entries[index] || null;
  };
  runtime.queueAIProvidersSync = function(delay) {
    if (typeof runtime.scheduleAIProvidersSync === 'function') {
      runtime.scheduleAIProvidersSync(delay);
    }
  };
  runtime.loadScript = function(name) {
    var cleaned = trimText(name).replace(/^\/+/, '');
    if (!cleaned) return Promise.resolve();
    if (runtime.pendingScripts[cleaned]) return runtime.pendingScripts[cleaned];

    runtime.pendingScripts[cleaned] = new Promise(function(resolve, reject) {
      if (document.querySelector('script[data-cpa-asset="' + cleaned + '"]')) {
        resolve();
        return;
      }
      var script = document.createElement('script');
      script.src = assetURL(cleaned);
      script.async = false;
      script.defer = false;
      script.setAttribute('data-cpa-asset', cleaned);
      script.onload = function() { resolve(); };
      script.onerror = function() { reject(new Error('failed to load ' + cleaned)); };
      (document.head || document.body || document.documentElement).appendChild(script);
    });

    return runtime.pendingScripts[cleaned];
  };

  window.CPAExtensions = runtime;

  if (!window.__cpaProviderFetchWrapped && runtime.nativeFetch) {
    window.__cpaProviderFetchWrapped = true;
    window.fetch = function(input, init) {
      var url = typeof input === 'string' ? input : (input && input.url ? input.url : '');
      var provider = runtime.matchProviderRequest(url);
      var method = trimText((init && init.method) || (input && input.method) || 'GET').toUpperCase();
      var nextInit = init;
      var headers = (init && init.headers) || (input && input.headers) || null;

      if (runtime.isManagementRequest(url) && runtime.rememberMgmtHeaders(headers) && location.hash.indexOf('#/ai-providers') === 0) {
        runtime.queueAIProvidersSync(80);
      }

      if (provider && typeof runtime.prepareProviderRequest === 'function') {
        var prepared = runtime.prepareProviderRequest(provider, method, input, init);
        if (prepared) {
          nextInit = prepared;
        }
      }

      var request = runtime.nativeFetch(input, nextInit);
      if (provider) {
        request.then(function(resp) {
          if (typeof runtime.afterProviderRequest === 'function') {
            runtime.afterProviderRequest(provider, method, resp);
          }
        }).catch(function() {});
      }
      return request;
    };
  }

  if (!window.__cpaProviderXHRWrapped && window.XMLHttpRequest && window.XMLHttpRequest.prototype) {
    window.__cpaProviderXHRWrapped = true;
    var nativeXHROpen = window.XMLHttpRequest.prototype.open;
    var nativeXHRSend = window.XMLHttpRequest.prototype.send;
    var nativeXHRSetRequestHeader = window.XMLHttpRequest.prototype.setRequestHeader;

    window.XMLHttpRequest.prototype.open = function(method, url) {
      this.__cpaUrl = url || '';
      this.__cpaMethod = trimText(method || 'GET').toUpperCase();
      this.__cpaHeaders = {};
      return nativeXHROpen.apply(this, arguments);
    };

    window.XMLHttpRequest.prototype.setRequestHeader = function(name, value) {
      if (!this.__cpaHeaders) this.__cpaHeaders = {};
      this.__cpaHeaders[name] = value;
      if (runtime.isManagementRequest(this.__cpaUrl) && runtime.rememberMgmtHeaders(this.__cpaHeaders) && location.hash.indexOf('#/ai-providers') === 0) {
        runtime.queueAIProvidersSync(80);
      }
      return nativeXHRSetRequestHeader.apply(this, arguments);
    };

    window.XMLHttpRequest.prototype.send = function() {
      var xhr = this;
      if (!xhr.__cpaLoadEndAttached) {
        xhr.__cpaLoadEndAttached = true;
        xhr.addEventListener('loadend', function() {
          if (!runtime.isManagementRequest(xhr.__cpaUrl)) return;
          var changed = runtime.rememberMgmtHeaders(xhr.__cpaHeaders || {});
          var provider = runtime.matchProviderRequest(xhr.__cpaUrl || '');
          if (provider && xhr.status >= 200 && xhr.status < 300) {
            try {
              runtime.syncProviderCache(provider, JSON.parse(xhr.responseText || '{}'));
            } catch (e) {}
            if (location.hash.indexOf('#/ai-providers') === 0) {
              runtime.queueAIProvidersSync(80);
            }
            return;
          }
          if (changed && location.hash.indexOf('#/ai-providers') === 0) {
            runtime.queueAIProvidersSync(80);
          }
        });
      }
      return nativeXHRSend.apply(this, arguments);
    };
  }

  runtime.loadScript('usage.plugin.js').catch(function() {});
  runtime.loadScript('ai-providers.plugin.js').catch(function() {});
})();
