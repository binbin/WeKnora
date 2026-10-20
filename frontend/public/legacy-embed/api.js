/**
 * IE9-safe XHR helpers for embed public APIs (same-origin only).
 */
(function (global) {
  'use strict';

  var LegacyEmbedApi = {};

  function createXHR() {
    if (typeof XMLHttpRequest !== 'undefined') {
      return new XMLHttpRequest();
    }
    throw new Error('XMLHttpRequest unavailable');
  }

  function parseJSON(text) {
    if (!text) return null;
    try {
      return JSON.parse(text);
    } catch (err) {
      return null;
    }
  }

  /**
   * @param {string} method
   * @param {string} url
   * @param {Object} opts
   * @param {Object=} opts.headers
   * @param {string=} opts.body
   * @param {function(Error|null, Object|null, XMLHttpRequest)=} opts.done
   */
  LegacyEmbedApi.request = function (method, url, opts) {
    opts = opts || {};
    var xhr = createXHR();
    var done = typeof opts.done === 'function' ? opts.done : function () {};
    xhr.open(method, url, true);
    xhr.setRequestHeader('Content-Type', 'application/json; charset=utf-8');
    xhr.setRequestHeader('Accept', 'application/json');
    var headers = opts.headers || {};
    var key;
    for (key in headers) {
      if (Object.prototype.hasOwnProperty.call(headers, key) && headers[key]) {
        xhr.setRequestHeader(key, headers[key]);
      }
    }
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) return;
      var body = parseJSON(xhr.responseText);
      if (xhr.status >= 200 && xhr.status < 300) {
        done(null, body, xhr);
        return;
      }
      var msg = (body && (body.error || body.message)) || ('HTTP ' + xhr.status);
      done(new Error(String(msg)), body, xhr);
    };
    xhr.send(opts.body || null);
    return xhr;
  };

  LegacyEmbedApi.authHeaders = function (token, sessionSig, visitorId) {
    var headers = { Authorization: 'Embed ' + token };
    if (sessionSig) headers['X-Embed-Session'] = sessionSig;
    if (visitorId) headers['X-Embed-Visitor'] = visitorId;
    return headers;
  };

  LegacyEmbedApi.bootstrapWeb = function (slug, done) {
    LegacyEmbedApi.request(
      'POST',
      '/api/v1/embed/web/' + encodeURIComponent(slug) + '/bootstrap',
      { body: '{}', done: done }
    );
  };

  LegacyEmbedApi.exchange = function (channelId, publishToken, done) {
    LegacyEmbedApi.request(
      'POST',
      '/api/v1/embed/' + encodeURIComponent(channelId) + '/exchange',
      {
        headers: { Authorization: 'Embed ' + publishToken },
        body: '{}',
        done: done,
      }
    );
  };

  LegacyEmbedApi.getConfig = function (channelId, token, done) {
    LegacyEmbedApi.request(
      'GET',
      '/api/v1/embed/' + encodeURIComponent(channelId) + '/config',
      { headers: LegacyEmbedApi.authHeaders(token), done: done }
    );
  };

  LegacyEmbedApi.createSession = function (channelId, token, done) {
    LegacyEmbedApi.request(
      'POST',
      '/api/v1/embed/' + encodeURIComponent(channelId) + '/sessions',
      {
        headers: LegacyEmbedApi.authHeaders(token),
        body: '{}',
        done: done,
      }
    );
  };

  LegacyEmbedApi.loadMessages = function (
    channelId, token, sessionId, sessionSig, limit, done
  ) {
    LegacyEmbedApi.request(
      'GET',
      '/api/v1/embed/' + encodeURIComponent(channelId) +
        '/messages/' + encodeURIComponent(sessionId) +
        '/load?limit=' + encodeURIComponent(String(limit || 50)),
      {
        headers: LegacyEmbedApi.authHeaders(token, sessionSig),
        done: done,
      }
    );
  };

  LegacyEmbedApi.suggestedQuestions = function (channelId, token, done) {
    LegacyEmbedApi.request(
      'GET',
      '/api/v1/embed/' + encodeURIComponent(channelId) + '/suggested-questions',
      { headers: LegacyEmbedApi.authHeaders(token), done: done }
    );
  };

  /**
   * Progressive SSE over XHR (IE9 readyState=3). Falls back via opts.onFail.
   * @param {Object} opts
   * @param {string} opts.url
   * @param {Object} opts.headers
   * @param {string} opts.body
   * @param {function(Object)} opts.onEvent
   * @param {function(Error=)} opts.onDone
   * @param {function(Error)} opts.onFail
   */
  LegacyEmbedApi.streamAgentChat = function (opts) {
    var xhr = createXHR();
    var consumed = 0;
    var buffer = '';
    var finished = false;
    var sawEvent = false;

    function finish(err) {
      if (finished) return;
      finished = true;
      if (err) {
        opts.onFail(err);
        return;
      }
      opts.onDone();
    }

    function consumeChunk(chunk) {
      buffer += chunk;
      var parts = buffer.split('\n');
      buffer = parts.pop();
      var index;
      for (index = 0; index < parts.length; index += 1) {
        var line = parts[index];
        if (line.indexOf('data:') !== 0) continue;
        var payload = line.slice(5);
        if (payload.charAt(0) === ' ') payload = payload.slice(1);
        payload = payload.replace(/\r$/, '');
        if (!payload || payload === '[DONE]') continue;
        try {
          var parsed = JSON.parse(payload);
          sawEvent = true;
          opts.onEvent(parsed);
        } catch (ignore) {
          // skip malformed SSE line
        }
      }
    }

    xhr.open('POST', opts.url, true);
    xhr.setRequestHeader('Content-Type', 'application/json; charset=utf-8');
    xhr.setRequestHeader('Accept', 'text/event-stream');
    var headers = opts.headers || {};
    var key;
    for (key in headers) {
      if (Object.prototype.hasOwnProperty.call(headers, key) && headers[key]) {
        xhr.setRequestHeader(key, headers[key]);
      }
    }
    xhr.onreadystatechange = function () {
      if (xhr.readyState === 3 || xhr.readyState === 4) {
        var text = xhr.responseText || '';
        if (text.length > consumed) {
          consumeChunk(text.slice(consumed));
          consumed = text.length;
        }
      }
      if (xhr.readyState !== 4) return;
      if (xhr.status < 200 || xhr.status >= 300) {
        finish(new Error('HTTP ' + xhr.status));
        return;
      }
      if (!sawEvent) {
        finish(new Error('empty stream'));
        return;
      }
      finish();
    };
    try {
      xhr.send(opts.body || '{}');
    } catch (err) {
      finish(err);
    }
    return xhr;
  };

  /**
   * Non-streaming backup: Accept application/json + stream=0.
   */
  LegacyEmbedApi.chatBuffered = function (
    channelId, token, sessionId, sessionSig, visitorId, query, agentId, kbIds, done
  ) {
    var body = {
      query: query,
      agent_enabled: true,
      channel: 'embed',
    };
    if (agentId) body.agent_id = agentId;
    if (kbIds && kbIds.length) body.knowledge_base_ids = kbIds;
    LegacyEmbedApi.request(
      'POST',
      '/api/v1/embed/' + encodeURIComponent(channelId) +
        '/agent-chat/' + encodeURIComponent(sessionId) + '?stream=0',
      {
        headers: LegacyEmbedApi.authHeaders(token, sessionSig, visitorId),
        body: JSON.stringify(body),
        done: done,
      }
    );
  };

  global.LegacyEmbedApi = LegacyEmbedApi;
})(typeof window !== 'undefined' ? window : this);
