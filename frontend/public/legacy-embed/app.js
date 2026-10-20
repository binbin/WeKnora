/**
 * IE8+ legacy guest/embed chat (jQuery 1.12).
 * Modern browsers keep Vue3 embed.html — this shell is sniff-only.
 */
(function (global, $) {
  'use strict';

  if (!$ || !global.LegacyEmbedApi || !global.LegacyEmbedMarkdown) {
    return;
  }

  var Api = global.LegacyEmbedApi;
  var Md = global.LegacyEmbedMarkdown;
  var STORAGE_PREFIX = 'weknora-embed-session:';
  var VISITOR_PREFIX = 'weknora-embed-visitor:';
  var SESSION_LIMIT = 40;

  var state = {
    channelId: '',
    token: '',
    sessionId: '',
    sessionSig: '',
    visitorId: '',
    config: null,
    multi: false,
    sessions: [],
    sending: false,
    hasMessages: false,
    hostContext: {},
    activeXhr: null,
    ieVersion: detectIeVersion(),
  };

  function detectIeVersion() {
    var mode = document.documentMode;
    if (typeof mode === 'number') return mode;
    var ua = navigator.userAgent || '';
    var match = /MSIE (\d+)/.exec(ua);
    return match ? parseInt(match[1], 10) : 0;
  }

  function preferBufferedChat() {
    // IE8 cannot reliably progressive-read SSE; use JSON backup as primary.
    return state.ieVersion > 0 && state.ieVersion < 9;
  }

  function parseRoute() {
    var params = {};
    var search = (location.search || '').replace(/^\?/, '');
    $.each(search.split('&'), function (_, pair) {
      if (!pair) return;
      var kv = pair.split('=');
      params[decodeURIComponent(kv[0] || '')] = decodeURIComponent(kv[1] || '');
    });
    var path = params.p || location.pathname || '';
    var hash = '';
    var hashIdx = path.indexOf('#');
    if (hashIdx >= 0) {
      hash = path.slice(hashIdx + 1);
      path = path.slice(0, hashIdx);
    }
    var qIdx = path.indexOf('?');
    var pathOnly = qIdx >= 0 ? path.slice(0, qIdx) : path;
    var pathQuery = qIdx >= 0 ? path.slice(qIdx + 1) : '';
    var token = params.token || '';
    if (!token && pathQuery) {
      $.each(pathQuery.split('&'), function (_, pair) {
        var kv = pair.split('=');
        if (decodeURIComponent(kv[0] || '') === 'token') {
          token = decodeURIComponent(kv[1] || '');
        }
      });
    }
    if (!token && hash) {
      $.each(hash.split('&'), function (_, pair) {
        var kv = pair.split('=');
        if (decodeURIComponent(kv[0] || '') === 'token') {
          token = decodeURIComponent(kv[1] || '');
        }
      });
    }
    var webMatch = pathOnly.match(/^\/w\/([^/]+)\/?$/);
    if (webMatch) {
      return { mode: 'web', slug: decodeURIComponent(webMatch[1]), token: token };
    }
    var embedMatch = pathOnly.match(/^\/embed\/([^/]+)\/?$/);
    if (embedMatch) {
      return {
        mode: 'embed',
        channelId: decodeURIComponent(embedMatch[1]),
        token: token,
      };
    }
    return { mode: 'unknown' };
  }

  function uuid() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (ch) {
      var r = (Math.random() * 16) | 0;
      var v = ch === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  }

  function getVisitorId(channelId) {
    var key = VISITOR_PREFIX + channelId;
    try {
      var existing = localStorage.getItem(key);
      if (existing) return existing;
      var id = uuid();
      localStorage.setItem(key, id);
      return id;
    } catch (err) {
      return uuid();
    }
  }

  function readSessions(channelId) {
    try {
      var raw = localStorage.getItem(STORAGE_PREFIX + channelId);
      if (!raw) return null;
      var parsed = JSON.parse(raw);
      if (!parsed || !parsed.sessions || !parsed.sessions.length) return null;
      return parsed;
    } catch (err) {
      return null;
    }
  }

  function writeSessions(channelId, currentId, sessions) {
    try {
      var list = (sessions || []).slice(0, SESSION_LIMIT);
      if (!list.length) {
        localStorage.removeItem(STORAGE_PREFIX + channelId);
        return;
      }
      localStorage.setItem(
        STORAGE_PREFIX + channelId,
        JSON.stringify({ currentId: currentId, sessions: list })
      );
    } catch (err) { /* private mode */ }
  }

  function formatTime(ts) {
    if (!ts) return '';
    var date = new Date(ts);
    var month = date.getMonth() + 1;
    var day = date.getDate();
    var hour = date.getHours();
    var minute = date.getMinutes();
    function pad(n) { return n < 10 ? '0' + n : String(n); }
    return month + '/' + day + ' ' + pad(hour) + ':' + pad(minute);
  }

  function postToParent(payload, sensitive) {
    if (global.parent === global) return;
    try {
      var target = '*';
      if (document.referrer) {
        var a = document.createElement('a');
        a.href = document.referrer;
        if (a.protocol && a.host) target = a.protocol + '//' + a.host;
      }
      if (sensitive && target === '*') return;
      global.parent.postMessage({
        source: 'weknora-embed',
        channel_id: state.channelId,
        type: payload.type,
        session_id: payload.session_id,
        query: payload.query,
        content: payload.content,
      }, target);
    } catch (err) { /* ignore */ }
  }

  function setError(msg) {
    $('#loading, #shell').hide();
    $('#error').html(Md.escapeHtml(msg || '加载失败')).show();
  }

  function applyTheme(config) {
    var color = (config && config.primary_color) || '#07C05F';
    $('#header-badge').css({ background: '#e8f8ef', color: color });
    $('.embed-session-sidebar__new, .embed-send-btn').css('background', color);
    $('#input-box').data('primary', color);
    if (config.page_title) document.title = config.page_title;
    var title = config.display_title || config.page_title || config.name ||
      config.agent_name || 'AI 助手';
    $('#header-title').text(title);
    document.title = title;
    if (config.agent_name && config.agent_name !== title) {
      $('#header-sub').text(config.agent_name).show();
    } else {
      $('#header-sub').hide();
    }
    if (config.agent_avatar) {
      $('#header-badge').text(config.agent_avatar);
    } else {
      $('#header-badge').text('AI');
    }
  }

  function updateNewChatEnabled() {
    var enabled = state.multi && state.hasMessages && !state.sending;
    $('#btn-new, #btn-header-new').prop('disabled', !enabled);
  }

  function setSendingUi(sending) {
    state.sending = sending;
    $('#btn-send').prop('disabled', sending || !$.trim($('#input').val()));
    if (sending) {
      $('#btn-send').hide();
      $('#btn-stop').show();
      if (!$('#typing').length) {
        $('#messages').append(
          '<div id="typing" class="embed-chat__typing">' +
            '<span class="embed-chat__typing-dot"></span>正在生成…</div>'
        );
      }
    } else {
      $('#btn-stop').hide();
      $('#btn-send').show();
      $('#typing').remove();
    }
    updateNewChatEnabled();
    scrollToBottom();
  }

  function scrollToBottom() {
    var sc = $('#scroll')[0];
    if (sc) sc.scrollTop = sc.scrollHeight;
  }

  function clearMessages() {
    $('#messages').empty();
    state.hasMessages = false;
    updateNewChatEnabled();
  }

  function appendWelcome(text) {
    if (!text) return;
    $('#messages').append(
      '<div class="embed-welcome-bubble">' +
        '<p class="embed-welcome-bubble__text">' + Md.escapeHtml(text) + '</p></div>'
    );
  }

  function appendMessage(role, text, opts) {
    opts = opts || {};
    var rowClass = role === 'user' ? 'message-row--user' : 'message-row--bot';
    var bubbleClass = role === 'user' ? 'embed-user-bubble' : 'embed-bot-bubble';
    var html;
    if (role === 'user' || opts.plain) {
      html = Md.renderPlain(text);
    } else {
      try { html = Md.render(text); } catch (err) { html = Md.renderPlain(text); }
    }
    var $row = $(
      '<div class="message-row ' + rowClass + '">' +
        '<div class="' + bubbleClass + '"></div></div>'
    );
    $row.find('.' + bubbleClass).html(html);
    $('#messages').append($row);
    if (role === 'user') {
      state.hasMessages = true;
      updateNewChatEnabled();
    }
    scrollToBottom();
    return $row.find('.' + bubbleClass);
  }

  function renderSessionList() {
    if (!state.multi) return;
    var $list = $('#session-list').empty();
    if (!state.sessions.length) {
      $list.append('<p class="embed-session-sidebar__empty">暂无历史对话</p>');
      return;
    }
    $.each(state.sessions, function (_, item) {
      var title = item.title || '新对话';
      var cls = item.id === state.sessionId ? ' is-active' : '';
      $list.append(
        '<button type="button" class="embed-session-item' + cls + '" data-id="' +
          Md.escapeHtml(item.id) + '" role="listitem">' +
          '<span class="embed-session-item__title">' + Md.escapeHtml(title) + '</span>' +
          '<span class="embed-session-item__meta">' + Md.escapeHtml(formatTime(item.updatedAt)) + '</span>' +
          '<span class="embed-session-item__delete" data-del="' + Md.escapeHtml(item.id) +
          '" title="删除对话">×</span></button>'
      );
    });
  }

  function renderSuggestions(questions) {
    $('#messages .embed-suggested').remove();
    if (!questions || !questions.length) return;
    if (state.hasMessages) return;
    var html = ['<div class="embed-suggested"><p class="embed-suggested__title">试试这样问</p>'];
    var i;
    for (i = 0; i < questions.length; i += 1) {
      var q = questions[i].question || questions[i];
      html.push(
        '<button type="button" class="embed-suggested__card" data-q="' +
          Md.escapeHtml(q) + '">' + Md.escapeHtml(q) + '</button>'
      );
    }
    html.push('</div>');
    $('#messages').append(html.join(''));
  }

  function loadSuggestions() {
    if (!state.config || state.config.show_suggested_questions === false) {
      $('#messages .embed-suggested').remove();
      return;
    }
    Api.suggestedQuestions(state.channelId, state.token, function (err, body) {
      if (err || !body || !body.data) return;
      renderSuggestions(body.data.questions || []);
    });
  }

  function loadHistory(done) {
    clearMessages();
    Api.loadMessages(
      state.channelId, state.token, state.sessionId, state.sessionSig, 50,
      function (err, body) {
        if (err) {
          if (done) done(err);
          return;
        }
        var list = (body && body.data) || [];
        var i;
        for (i = 0; i < list.length; i += 1) {
          var msg = list[i];
          appendMessage(msg.role === 'user' ? 'user' : 'bot', msg.content || '');
        }
        if (!list.length && state.config && state.config.welcome_message) {
          appendWelcome(state.config.welcome_message);
          loadSuggestions();
        }
        state.hasMessages = list.length > 0;
        updateNewChatEnabled();
        if (done) done(null);
      }
    );
  }

  function activateSession(entry, opts) {
    opts = opts || {};
    state.sessionId = entry.id;
    state.sessionSig = entry.sig || '';
    if (state.multi) {
      var found = false;
      var i;
      for (i = 0; i < state.sessions.length; i += 1) {
        if (state.sessions[i].id === entry.id) {
          state.sessions[i] = entry;
          found = true;
          break;
        }
      }
      if (!found) state.sessions.unshift(entry);
      writeSessions(state.channelId, entry.id, state.sessions);
      renderSessionList();
    }
    if (!opts.skipHistory) loadHistory();
  }

  function createSession(done) {
    Api.createSession(state.channelId, state.token, function (err, body) {
      if (err || !body || !body.data || !body.data.id) {
        if (done) done(err || new Error('session failed'));
        return;
      }
      var entry = {
        id: body.data.id,
        sig: body.data.sig || '',
        agentId: (state.config && state.config.agent_id) || '',
        updatedAt: new Date().getTime(),
      };
      activateSession(entry, { skipHistory: true });
      clearMessages();
      if (state.config && state.config.welcome_message) {
        appendWelcome(state.config.welcome_message);
      }
      loadSuggestions();
      if (done) done(null);
    });
  }

  function removeSession(targetId) {
    if (!state.multi || !targetId) return;
    var remaining = [];
    var i;
    for (i = 0; i < state.sessions.length; i += 1) {
      if (state.sessions[i].id !== targetId) remaining.push(state.sessions[i]);
    }
    state.sessions = remaining;
    if (!remaining.length) {
      writeSessions(state.channelId, '', []);
      createSession();
      return;
    }
    var next = remaining[0];
    if (targetId === state.sessionId) {
      writeSessions(state.channelId, next.id, remaining);
      activateSession(next);
    } else {
      writeSessions(state.channelId, state.sessionId, remaining);
      renderSessionList();
    }
  }

  function touchSessionTitle(query) {
    if (!state.multi) return;
    var i;
    for (i = 0; i < state.sessions.length; i += 1) {
      if (state.sessions[i].id === state.sessionId) {
        if (!state.sessions[i].title) {
          state.sessions[i].title = query.slice(0, 40);
        }
        state.sessions[i].updatedAt = new Date().getTime();
        break;
      }
    }
    writeSessions(state.channelId, state.sessionId, state.sessions);
    renderSessionList();
  }

  function eventType(evt) {
    return evt.response_type || evt.type || '';
  }

  function sendQuery(query) {
    var text = $.trim(String(query || ''));
    if (!text || state.sending) return;
    $('#messages .embed-suggested').remove();
    appendMessage('user', text);
    $('#input').val('');
    setSendingUi(true);

    var $bot = appendMessage('bot', '…', { plain: true });
    var answer = '';
    var sawMcp = false;
    var kbIds = (state.config && state.config.knowledge_base_ids) || [];
    var agentId = (state.config && state.config.agent_id) || '';

    function finishOk(finalText) {
      state.activeXhr = null;
      setSendingUi(false);
      $bot.html(Md.render(finalText || answer || ''));
      postToParent({
        type: 'message_received',
        session_id: state.sessionId,
        content: finalText || answer,
      }, true);
      if (sawMcp) $('#mcp-hint').show();
      touchSessionTitle(text);
      scrollToBottom();
    }

    function finishErr(err) {
      state.activeXhr = null;
      setSendingUi(false);
      $bot.html(Md.renderPlain(
        '发送失败：' + (err && err.message ? err.message : '请重试')
      ));
    }

    function runBuffered() {
      Api.chatBuffered(
        state.channelId, state.token, state.sessionId, state.sessionSig,
        state.visitorId, text, agentId, kbIds,
        function (err2, body2) {
          if (err2) { finishErr(err2); return; }
          var data = (body2 && body2.data) || {};
          answer = data.answer || '';
          finishOk(answer);
        }
      );
    }

    postToParent({
      type: 'message_sent',
      session_id: state.sessionId,
      query: text,
    }, true);

    if (preferBufferedChat()) {
      runBuffered();
      return;
    }

    var body = {
      query: text,
      agent_enabled: true,
      channel: 'embed',
    };
    if (agentId) body.agent_id = agentId;
    if (kbIds.length) body.knowledge_base_ids = kbIds;

    var url =
      '/api/v1/embed/' + encodeURIComponent(state.channelId) +
      '/agent-chat/' + encodeURIComponent(state.sessionId);

    state.activeXhr = Api.streamAgentChat({
      url: url,
      headers: Api.authHeaders(state.token, state.sessionSig, state.visitorId),
      body: JSON.stringify(body),
      onEvent: function (evt) {
        var kind = eventType(evt);
        if (kind === 'answer' || kind === 'final_answer') {
          var chunk = evt.content || '';
          if (typeof chunk === 'string' && chunk) {
            answer += chunk;
            $bot.html(Md.render(answer));
            scrollToBottom();
          }
        }
        if (
          kind === 'mcp_oauth' || kind === 'tool_approval' ||
          kind === 'pending_tool_approval'
        ) {
          sawMcp = true;
        }
        if (kind === 'error') {
          answer = answer || (evt.content || evt.error || '出错了');
        }
      },
      onDone: function () { finishOk(answer); },
      onFail: function () { runBuffered(); },
    });
  }

  function finishBootstrap(channelId, token, config) {
    state.channelId = channelId;
    state.token = token;
    state.config = config || {};
    state.visitorId = getVisitorId(channelId);
    applyTheme(state.config);

    $('#loading').hide();
    $('#shell').show();

    if (state.multi) {
      $('#shell').addClass('is-multi');
      $('#btn-header-new').show();
      layoutSidebar();
    }

    function ready() {
      postToParent({ type: 'ready' }, false);
    }

    if (state.multi) {
      var stored = readSessions(channelId);
      if (stored && stored.sessions && stored.sessions.length) {
        state.sessions = stored.sessions;
        var current = null;
        var i;
        for (i = 0; i < stored.sessions.length; i += 1) {
          if (stored.sessions[i].id === stored.currentId) {
            current = stored.sessions[i];
            break;
          }
        }
        if (!current) current = stored.sessions[0];
        activateSession(current);
        ready();
        return;
      }
    }

    createSession(function (err) {
      if (err) {
        setError('创建会话失败，请刷新重试');
        return;
      }
      ready();
    });
  }

  function bootstrapWeb(slug) {
    state.multi = true;
    Api.bootstrapWeb(slug, function (err, body) {
      if (err) {
        setError(err.message || '链接无效或已禁用');
        return;
      }
      var data = body && body.data;
      if (!data || !data.channel_id || !data.session_token || !data.config) {
        setError('链接无效');
        return;
      }
      finishBootstrap(data.channel_id, data.session_token, data.config);
    });
  }

  function isSessionToken(token) {
    return typeof token === 'string' && token.indexOf('ems_') === 0;
  }

  function bootstrapEmbed(channelId, token) {
    state.multi = false;
    state.channelId = channelId;

    function afterToken(apiToken) {
      Api.getConfig(channelId, apiToken, function (err, body) {
        if (err || !body || !body.data) {
          setError((err && err.message) || '渠道无效或未授权');
          return;
        }
        finishBootstrap(channelId, apiToken, body.data);
      });
    }

    if (!token) {
      $('#loading').text('等待宿主授权…').show();
      postToParent({ type: 'bootstrap_request' }, false);
      return;
    }
    if (isSessionToken(token)) {
      afterToken(token);
      return;
    }
    Api.exchange(channelId, token, function (err, body) {
      if (err || !body || !body.data || !body.data.session_token) {
        setError((err && err.message) || '授权失败');
        return;
      }
      afterToken(body.data.session_token);
    });
  }

  function layoutSidebar() {
    var narrow = $(window).width() <= 768;
    if (narrow) {
      $('#shell').addClass('is-drawer-mode');
      $('#sidebar').addClass('is-drawer').removeClass('is-open');
      $('#btn-menu').show();
    } else {
      $('#shell').removeClass('is-drawer-mode');
      $('#sidebar').removeClass('is-drawer is-open').show();
      $('#btn-menu').hide();
      $('#sidebar-mask').hide();
    }
  }

  function onHostMessage(event) {
    var data = event.originalEvent ? event.originalEvent.data : event.data;
    if (!data || data.source !== 'weknora-host') return;
    if (data.type === 'provide_token' && data.token) {
      if (!state.token) {
        bootstrapEmbed(state.channelId || data.channel_id, data.token);
      }
    }
    if (data.type === 'set_context' && data.payload) {
      state.hostContext = data.payload;
    }
    if (data.type === 'open_with_query' && data.payload && data.payload.query) {
      sendQuery(data.payload.query);
    }
  }

  function bindEvents() {
    $('#btn-send').on('click', function () { sendQuery($('#input').val()); });
    $('#btn-stop').on('click', function () {
      try {
        if (state.activeXhr && state.activeXhr.abort) state.activeXhr.abort();
      } catch (err) { /* ignore */ }
      setSendingUi(false);
    });
    $('#input').on('keydown', function (evt) {
      var key = evt.keyCode || evt.which;
      if (key === 13 && !evt.shiftKey) {
        evt.preventDefault();
        sendQuery($('#input').val());
      }
    }).on('focus', function () {
      $('#input-box').addClass('is-focus');
    }).on('blur', function () {
      $('#input-box').removeClass('is-focus');
    }).on('input keyup', function () {
      if (!state.sending) {
        $('#btn-send').prop('disabled', !$.trim($('#input').val()));
      }
    });

    $('#btn-new, #btn-header-new').on('click', function () {
      if (!state.multi || state.sending || !state.hasMessages) return;
      createSession();
      if ($('#shell').hasClass('is-drawer-mode')) {
        $('#sidebar').removeClass('is-open');
        $('#sidebar-mask').hide();
      }
    });

    $('#btn-menu').on('click', function () {
      var open = !$('#sidebar').hasClass('is-open');
      $('#sidebar').toggleClass('is-open', open);
      $('#sidebar-mask').toggle(open);
    });
    $('#sidebar-mask').on('click', function () {
      $('#sidebar').removeClass('is-open');
      $('#sidebar-mask').hide();
    });

    $('#session-list').on('click', '.embed-session-item__delete', function (evt) {
      evt.preventDefault();
      evt.stopPropagation();
      removeSession($(this).attr('data-del'));
    });
    $('#session-list').on('click', '.embed-session-item', function (evt) {
      if ($(evt.target).hasClass('embed-session-item__delete')) return;
      var id = $(this).attr('data-id');
      if (!id || id === state.sessionId) return;
      var i;
      for (i = 0; i < state.sessions.length; i += 1) {
        if (state.sessions[i].id === id) {
          activateSession(state.sessions[i]);
          break;
        }
      }
      if ($('#shell').hasClass('is-drawer-mode')) {
        $('#sidebar').removeClass('is-open');
        $('#sidebar-mask').hide();
      }
    });

    $('#messages').on('click', '.embed-suggested__card', function () {
      sendQuery($(this).attr('data-q') || $(this).text());
    });

    $(global).on('message', onHostMessage);
    $(global).on('resize', function () {
      if (state.multi) layoutSidebar();
    });
  }

  function boot() {
    if (state.ieVersion > 0 && state.ieVersion < 9) {
      $('html').addClass('ie8');
    }
    // IE8: SVG img often fails — show text label.
    if (!$('#btn-send .embed-send-btn__text').length) {
      $('#btn-send').append('<span class="embed-send-btn__text">发送</span>');
    }
    $('#btn-send').prop('disabled', true);
    bindEvents();

    var route = parseRoute();
    if (route.mode === 'web') {
      bootstrapWeb(route.slug);
      return;
    }
    if (route.mode === 'embed') {
      bootstrapEmbed(route.channelId, route.token);
      return;
    }
    setError('无效的嵌入地址');
  }

  $(boot);
})(typeof window !== 'undefined' ? window : this, window.jQuery);
