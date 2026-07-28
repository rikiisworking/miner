(function () {
  var video = document.querySelector('[data-testid="camera-video"]');
  var canvas = document.querySelector('[data-testid="camera-canvas"]');
  var freezeImg = document.querySelector('[data-testid="freeze-image"]');
  var boxesEl = document.querySelector('[data-testid="sentence-boxes"]');
  var chipsEl = document.querySelector('[data-testid="sentence-chips"]');
  var errEl = document.querySelector('[data-testid="camera-error"]');
  var statusEl = document.querySelector('[data-testid="capture-status"]');
  var shutter = document.querySelector('[data-testid="camera-shutter"]');
  var shutterWrap = document.querySelector('[data-testid="shutter-wrap"]');
  var backBtn = document.querySelector('[data-testid="capture-back"]');
  var detailEl = document.querySelector('[data-testid="sentence-detail"]');
  var detailBack = document.querySelector('[data-testid="detail-back"]');
  var analyzeSlot = document.querySelector('[data-testid="analyze-result"]');
  var capturePage = document.querySelector('[data-testid="capture-page"]');

  var stream = null;
  /** @type {'live'|'frozen'|'detail'} */
  var mode = 'live';
  var freezeDataUrl = '';
  var regions = [];
  var candidates = [];
  var capturing = false;
  var analyzing = false;

  function showError(msg) {
    if (!errEl) return;
    errEl.textContent = msg || '';
    errEl.hidden = !msg;
  }

  function clearError() {
    showError('');
  }

  function setStatus(msg) {
    if (!statusEl) return;
    if (msg) {
      statusEl.textContent = msg;
      statusEl.hidden = false;
    } else {
      statusEl.hidden = true;
    }
  }

  function cameraAPIAvailable() {
    return !!(navigator.mediaDevices && typeof navigator.mediaDevices.getUserMedia === 'function');
  }

  function getUserMediaTimed(constraints, ms) {
    return new Promise(function (resolve, reject) {
      var settled = false;
      var timer = setTimeout(function () {
        if (settled) return;
        settled = true;
        var err = new Error('Camera open timed out');
        err.name = 'TimeoutError';
        reject(err);
      }, ms);
      navigator.mediaDevices.getUserMedia(constraints).then(function (s) {
        if (settled) {
          s.getTracks().forEach(function (t) { t.stop(); });
          return;
        }
        settled = true;
        clearTimeout(timer);
        resolve(s);
      }, function (err) {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        reject(err);
      });
    });
  }

  function stopStream() {
    if (stream) {
      stream.getTracks().forEach(function (t) { t.stop(); });
      stream = null;
    }
    if (video) video.srcObject = null;
  }

  function startCamera() {
    clearError();
    if (!cameraAPIAvailable()) {
      showError('Camera not available in this browser. Use HTTPS (tunnel) on phone.');
      return;
    }
    if (shutter) shutter.disabled = true;
    getUserMediaTimed({
      audio: false,
      video: {
        facingMode: { ideal: 'environment' },
        width: { ideal: 1920 },
        height: { ideal: 1080 }
      }
    }, 4000).then(function (s) {
      stream = s;
      if (video) {
        video.srcObject = s;
        video.hidden = false;
        var p = video.play();
        if (p && typeof p.catch === 'function') p.catch(function () {});
      }
      if (shutter) shutter.disabled = false;
    }).catch(function (err) {
      if (shutter) shutter.disabled = false;
      var name = (err && err.name) || '';
      if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
        showError('Camera permission denied. Allow camera for this site.');
      } else if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
        showError('No camera found.');
      } else if (name === 'NotReadableError' || name === 'TrackStartError') {
        showError('Camera is busy. Close other apps and try again.');
      } else if (name === 'TimeoutError') {
        showError('Camera did not open in time. Try again.');
      } else {
        showError('Could not open camera.');
      }
    });
  }

  function clearOverlays() {
    if (boxesEl) {
      boxesEl.innerHTML = '';
      boxesEl.hidden = true;
    }
    if (chipsEl) {
      chipsEl.innerHTML = '';
      chipsEl.classList.remove('show');
    }
    regions = [];
    candidates = [];
  }

  function enterLive() {
    mode = 'live';
    freezeDataUrl = '';
    capturing = false;
    analyzing = false;
    clearOverlays();
    if (freezeImg) {
      freezeImg.removeAttribute('src');
      freezeImg.hidden = true;
    }
    if (video) video.hidden = false;
    if (shutterWrap) shutterWrap.hidden = false;
    if (shutter) shutter.disabled = false;
    if (detailEl) {
      detailEl.classList.remove('show');
      detailEl.hidden = true;
    }
    if (capturePage) capturePage.style.display = '';
    document.documentElement.style.overflow = 'hidden';
    document.body.style.overflow = 'hidden';
    setStatus('');
    clearError();
    if (!stream) startCamera();
  }

  function enterFrozen(dataUrl, regs, cands) {
    mode = 'frozen';
    freezeDataUrl = dataUrl || freezeDataUrl;
    regions = regs || [];
    candidates = cands || [];
    capturing = false;
    analyzing = false;
    stopStream();
    if (video) video.hidden = true;
    if (freezeImg && freezeDataUrl) {
      freezeImg.src = freezeDataUrl;
      freezeImg.hidden = false;
    }
    if (shutterWrap) shutterWrap.hidden = true;
    if (detailEl) {
      detailEl.classList.remove('show');
      detailEl.hidden = true;
    }
    if (capturePage) capturePage.style.display = '';
    document.documentElement.style.overflow = 'hidden';
    document.body.style.overflow = 'hidden';
    setStatus('');
    renderRegions();
  }

  function enterDetail() {
    mode = 'detail';
    if (detailEl) {
      detailEl.hidden = false;
      detailEl.classList.add('show');
    }
    document.documentElement.style.overflow = '';
    document.body.style.overflow = '';
  }

  function regionTexts() {
    var set = {};
    (regions || []).forEach(function (r) {
      if (r && r.text) set[r.text] = true;
    });
    return set;
  }

  function renderChips(list) {
    if (!chipsEl || !list || !list.length) return;
    chipsEl.classList.add('show');
    list.forEach(function (text, i) {
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.setAttribute('data-testid', 'sentence-chip');
      btn.setAttribute('data-index', String(i));
      btn.lang = 'ja';
      btn.textContent = text;
      btn.addEventListener('click', function () {
        pickSentence(text);
      });
      chipsEl.appendChild(btn);
    });
  }

  function renderRegions() {
    if (!boxesEl || !chipsEl) return;
    boxesEl.innerHTML = '';
    chipsEl.innerHTML = '';
    chipsEl.classList.remove('show');

    if (regions.length > 0) {
      boxesEl.hidden = false;
      regions.forEach(function (r, i) {
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.setAttribute('data-testid', 'sentence-region');
        btn.setAttribute('data-index', String(i));
        btn.setAttribute('aria-label', r.text || ('Sentence ' + (i + 1)));
        btn.style.left = (Math.max(0, r.x) * 100) + '%';
        btn.style.top = (Math.max(0, r.y) * 100) + '%';
        btn.style.width = (Math.max(0.04, r.w) * 100) + '%';
        btn.style.height = (Math.max(0.04, r.h) * 100) + '%';
        btn.addEventListener('click', function () {
          pickSentence(r.text);
        });
        boxesEl.appendChild(btn);
      });
      // Chips for candidates not covered by a region (partial geometry).
      var covered = regionTexts();
      var missing = (candidates || []).filter(function (t) { return t && !covered[t]; });
      if (missing.length) renderChips(missing);
      return;
    }

    // No geometry → all candidates as chips.
    renderChips(candidates || []);
  }

  function pickSentence(sentence) {
    if (!sentence || analyzing) return;
    analyzing = true;
    clearError();
    setStatus('Analyzing…');
    var body = new URLSearchParams();
    body.set('sentence', sentence);
    fetch('/analyze', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'HX-Request': 'true',
        'Accept': 'text/html'
      },
      body: body.toString()
    }).then(function (res) {
      return res.text().then(function (html) {
        return { ok: res.ok, status: res.status, html: html };
      });
    }).then(function (r) {
      analyzing = false;
      setStatus('');
      if (!r.ok) {
        // 4xx analyze still returns error fragment — show in detail only for 422 with fragment.
        if (r.status === 422 || r.status === 400) {
          if (analyzeSlot) {
            analyzeSlot.innerHTML = r.html;
            if (window.htmx) htmx.process(analyzeSlot);
          }
          enterDetail();
          try {
            history.pushState({ miner: 'detail' }, '', '#detail');
          } catch (e) { /* ignore */ }
          return;
        }
        showError('Could not analyze. Try another sentence.');
        return;
      }
      if (analyzeSlot) {
        analyzeSlot.innerHTML = r.html;
        if (window.htmx) htmx.process(analyzeSlot);
      }
      enterDetail();
      try {
        history.pushState({ miner: 'detail' }, '', '#detail');
      } catch (e) { /* ignore */ }
    }).catch(function () {
      analyzing = false;
      setStatus('');
      showError('Could not analyze. Try another sentence.');
    });
  }

  function captureFrame() {
    clearError();
    if (capturing || mode !== 'live') return;
    if (!video || !canvas) {
      showError('Camera not ready.');
      return;
    }
    var w = video.videoWidth || 0;
    var h = video.videoHeight || 0;
    if (w < 2 || h < 2) {
      showError('Camera preview not ready yet. Wait a moment.');
      return;
    }
    capturing = true;
    if (shutter) shutter.disabled = true;
    canvas.width = w;
    canvas.height = h;
    var ctx = canvas.getContext('2d');
    if (!ctx) {
      capturing = false;
      if (shutter) shutter.disabled = false;
      showError('Could not capture frame.');
      return;
    }
    ctx.drawImage(video, 0, 0, w, h);
    setStatus('Reading page…');
    canvas.toBlob(function (blob) {
      if (!blob) {
        capturing = false;
        if (shutter) shutter.disabled = false;
        setStatus('');
        showError('Capture failed. Try again.');
        return;
      }
      var dataUrl = canvas.toDataURL('image/jpeg', 0.88);
      var fd = new FormData();
      fd.append('image', blob, 'capture.jpg');
      fetch('/ingest', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Accept': 'application/json' },
        body: fd
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, status: res.status, data: data };
        }).catch(function () {
          return { ok: false, status: res.status, data: { error: 'Bad response from server.' } };
        });
      }).then(function (r) {
        capturing = false;
        if (shutter) shutter.disabled = false;
        setStatus('');
        if (!r.ok) {
          showError((r.data && r.data.error) || 'Could not read page. Retake photo.');
          return;
        }
        enterFrozen(dataUrl, r.data.regions || [], r.data.candidates || []);
      }).catch(function () {
        capturing = false;
        if (shutter) shutter.disabled = false;
        setStatus('');
        showError('Network error. Try again.');
      });
    }, 'image/jpeg', 0.88);
  }

  function onBack() {
    clearError();
    if (mode === 'detail') {
      enterFrozen(freezeDataUrl, regions, candidates);
      try {
        if (location.hash === '#detail') history.replaceState(null, '', location.pathname);
      } catch (e) { /* ignore */ }
      return;
    }
    if (mode === 'frozen') {
      enterLive();
      return;
    }
    stopStream();
    window.location.href = '/home';
  }

  if (shutter) {
    shutter.addEventListener('click', function () {
      if (mode !== 'live' || capturing) return;
      captureFrame();
    });
  }
  if (backBtn) backBtn.addEventListener('click', onBack);
  if (detailBack) detailBack.addEventListener('click', onBack);

  window.addEventListener('popstate', function () {
    if (mode === 'detail') {
      enterFrozen(freezeDataUrl, regions, candidates);
    }
  });

  window.addEventListener('pagehide', stopStream);
  window.addEventListener('beforeunload', stopStream);

  if (capturePage) {
    document.documentElement.style.overflow = 'hidden';
    document.body.style.overflow = 'hidden';
    enterLive();
    window.minerCapture = {
      enterFrozen: enterFrozen,
      enterLive: enterLive,
      pickSentence: pickSentence,
      mode: function () { return mode; }
    };
  }
})();
