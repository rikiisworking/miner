(function () {
  var startBtn = document.querySelector('[data-testid="camera-start"]');
  var stopBtn = document.querySelector('[data-testid="camera-stop"]');
  var shutterBtn = document.querySelector('[data-testid="camera-shutter"]');
  var live = document.querySelector('[data-testid="camera-live"]');
  var video = document.querySelector('[data-testid="camera-video"]');
  var canvas = document.querySelector('[data-testid="camera-canvas"]');
  var errEl = document.querySelector('[data-testid="camera-error"]');
  var fallbackEl = document.querySelector('[data-testid="camera-fallback"]');
  var photoInput = document.querySelector('[data-testid="photo-input"]');
  var photoForm = document.querySelector('[data-testid="photo-upload-form"]');
  var stream = null;

  function showError(msg) {
    if (!errEl) return;
    errEl.textContent = msg;
    errEl.hidden = false;
  }

  function clearError() {
    if (!errEl) return;
    errEl.textContent = '';
    errEl.hidden = true;
  }

  function cameraAPIAvailable() {
    return !!(navigator.mediaDevices && typeof navigator.mediaDevices.getUserMedia === 'function');
  }

  // Headless / stuck permission prompts must not hang the UI forever.
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
          // Timed out already — release late stream.
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
    if (live) live.hidden = true;
    if (startBtn) {
      startBtn.hidden = false;
      startBtn.disabled = false;
    }
  }

  function showFallback() {
    if (fallbackEl) fallbackEl.hidden = false;
  }

  if (!cameraAPIAvailable()) {
    showFallback();
  }

  if (startBtn) {
    startBtn.addEventListener('click', function () {
      clearError();
      if (!cameraAPIAvailable()) {
        showFallback();
        showError('Camera not available. Use page photo upload instead.');
        return;
      }
      startBtn.disabled = true;
      getUserMediaTimed({
        audio: false,
        video: {
          facingMode: { ideal: 'environment' },
          width: { ideal: 1920 },
          height: { ideal: 1080 }
        }
      }, 4000).then(function (s) {
        stream = s;
        video.srcObject = s;
        live.hidden = false;
        startBtn.hidden = true;
        startBtn.disabled = false;
        // Ensure playback on iOS Safari
        var p = video.play();
        if (p && typeof p.catch === 'function') p.catch(function () {});
      }).catch(function (err) {
        startBtn.disabled = false;
        var name = (err && err.name) || '';
        if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
          showError('Camera permission denied. Allow camera for this site, or use page photo upload.');
        } else if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
          showError('No camera found. Use page photo upload instead.');
        } else if (name === 'NotReadableError' || name === 'TrackStartError') {
          showError('Camera is busy or unreadable. Close other apps, or use page photo upload.');
        } else if (name === 'TimeoutError') {
          showError('Camera did not open in time. Use page photo upload instead.');
        } else {
          showError('Could not open camera. Use page photo upload instead.');
        }
        showFallback();
      });
    });
  }

  if (stopBtn) {
    stopBtn.addEventListener('click', function () {
      clearError();
      stopStream();
    });
  }

  if (shutterBtn) {
    shutterBtn.addEventListener('click', function () {
      clearError();
      if (!stream || !video || !canvas || !photoInput || !photoForm) {
        showError('Camera not ready. Open camera first, or use page photo upload.');
        return;
      }
      var w = video.videoWidth || 0;
      var h = video.videoHeight || 0;
      if (w < 2 || h < 2) {
        showError('Camera preview not ready yet. Wait a moment and try again.');
        return;
      }
      canvas.width = w;
      canvas.height = h;
      var ctx = canvas.getContext('2d');
      if (!ctx) {
        showError('Could not capture frame. Use page photo upload instead.');
        return;
      }
      ctx.drawImage(video, 0, 0, w, h);
      shutterBtn.disabled = true;
      canvas.toBlob(function (blob) {
        shutterBtn.disabled = false;
        if (!blob) {
          showError('Capture failed. Try again or use page photo upload.');
          return;
        }
        try {
          var file = new File([blob], 'capture.jpg', { type: blob.type || 'image/jpeg' });
          var dt = new DataTransfer();
          dt.items.add(file);
          photoInput.files = dt.files;
          // Reuse ticket-06 HTMX multipart form → same /ingest pipeline.
          if (typeof photoForm.requestSubmit === 'function') {
            photoForm.requestSubmit();
          } else {
            photoForm.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
          }
        } catch (e) {
          showError('Could not hand off capture to upload. Use page photo upload instead.');
        }
      }, 'image/jpeg', 0.92);
    });
  }

  window.addEventListener('pagehide', stopStream);
  window.addEventListener('beforeunload', stopStream);
})();
