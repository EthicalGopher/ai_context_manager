/* Media viewer shared by the app and the exported HTML page: a video player
 * with smooth seeking, speed, volume, fullscreen, and zoom/pan for videos and
 * images. Plain JS with no imports so it can be embedded in a standalone file.
 * Usage: <div class="mv" data-kind="video|image" data-src="..." data-caption="..."></div>
 * then window.aicwMountViewer(el) or window.aicwMountAll(document). */
(function () {
  var SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2]
  var MIN = 1
  var MAX = 8

  function el(tag, cls, attrs) {
    var n = document.createElement(tag)
    if (cls) n.className = cls
    for (var k in attrs || {}) n.setAttribute(k, attrs[k])
    return n
  }
  function btn(label, text, cls) {
    var b = el('button', 'mv-btn ' + (cls || ''), { type: 'button', 'aria-label': label, title: label })
    b.textContent = text
    return b
  }
  function fmt(t) {
    if (!isFinite(t)) return '0:00'
    t = Math.max(0, Math.floor(t))
    var h = Math.floor(t / 3600), m = Math.floor((t % 3600) / 60), s = t % 60
    return (h ? h + ':' + (m < 10 ? '0' : '') : '') + m + ':' + (s < 10 ? '0' : '') + s
  }
  function clamp(v, a, b) { return Math.min(b, Math.max(a, v)) }

  window.aicwMountViewer = function (root) {
    if (root.__mv) return root.__mv
    var kind = root.getAttribute('data-kind') === 'image' ? 'image' : 'video'
    var src = root.getAttribute('data-src') || ''
    var caption = root.getAttribute('data-caption') || ''
    var cleanups = []
    function on(t, ev, fn, opt) { t.addEventListener(ev, fn, opt); cleanups.push(function () { t.removeEventListener(ev, fn, opt) }) }

    root.textContent = ''
    root.classList.add('mv', 'mv-' + kind)
    root.tabIndex = 0
    root.setAttribute('role', 'group')
    root.setAttribute('aria-label', (kind === 'video' ? 'Video player' : 'Image viewer') + (caption ? ': ' + caption : ''))

    var stage = el('div', 'mv-stage')
    var layer = el('div', 'mv-layer')
    var media = el(kind === 'video' ? 'video' : 'img', 'mv-media')
    if (kind === 'video') {
      media.preload = 'auto'
      media.setAttribute('playsinline', '')
      media.disablePictureInPicture = false
    } else {
      media.alt = caption || 'image'
      media.draggable = false
    }
    layer.appendChild(media)
    stage.appendChild(layer)
    if (caption) { var cap = el('div', 'mv-caption'); cap.textContent = caption; stage.appendChild(cap) }
    var spin = el('div', 'mv-spin', { 'aria-hidden': 'true' })
    var msg = el('div', 'mv-msg', { role: 'alert' })
    stage.appendChild(spin)
    stage.appendChild(msg)
    root.appendChild(stage)

    // ---- zoom and pan ----
    var k = 1, x = 0, y = 0
    var zoomLabel = el('button', 'mv-btn mv-zoomval', { type: 'button', 'aria-label': 'Reset zoom', title: 'Reset zoom (0)' })
    function size() { return { w: stage.clientWidth || 1, h: stage.clientHeight || 1 } }
    function apply() {
      var s = size()
      x = clamp(x, s.w - s.w * k, 0)
      y = clamp(y, s.h - s.h * k, 0)
      if (k === 1) { x = 0; y = 0 }
      layer.style.transform = 'translate3d(' + x + 'px,' + y + 'px,0) scale(' + k + ')'
      zoomLabel.textContent = Math.round(k * 100) + '%'
      stage.classList.toggle('mv-zoomed', k > 1)
    }
    function zoomAt(cx, cy, nk) {
      nk = clamp(nk, MIN, MAX)
      x = cx - (cx - x) * nk / k
      y = cy - (cy - y) * nk / k
      k = nk
      apply()
    }
    function zoomBy(f) { var s = size(); zoomAt(s.w / 2, s.h / 2, k * f) }
    function resetZoom() { k = 1; apply() }
    function localPoint(e) { var r = stage.getBoundingClientRect(); return { x: e.clientX - r.left, y: e.clientY - r.top } }

    var zoomOut = btn('Zoom out (-)', '−')
    var zoomIn = btn('Zoom in (+)', '+')
    on(zoomOut, 'click', function () { zoomBy(1 / 1.25) })
    on(zoomIn, 'click', function () { zoomBy(1.25) })
    on(zoomLabel, 'click', resetZoom)

    // Ctrl/Cmd + wheel (and trackpad pinch) zooms at the pointer; plain wheel still scrolls the page.
    on(stage, 'wheel', function (e) {
      if (!e.ctrlKey && !e.metaKey) return
      e.preventDefault()
      var p = localPoint(e)
      zoomAt(p.x, p.y, k * Math.exp(-e.deltaY * 0.01))
    }, { passive: false })

    var ptrs = {}, pinch = 0, moved = 0, startX = 0, startY = 0, panning = false
    function dist() { var a = Object.keys(ptrs).map(function (i) { return ptrs[i] }); return a.length < 2 ? 0 : Math.hypot(a[0].x - a[1].x, a[0].y - a[1].y) }
    on(stage, 'pointerdown', function (e) {
      if (e.button > 0) return
      var p = localPoint(e)
      ptrs[e.pointerId] = p
      try { stage.setPointerCapture(e.pointerId) } catch (_) { /* not capturable */ }
      moved = 0; startX = p.x; startY = p.y; panning = k > 1
      pinch = dist()
    })
    on(stage, 'pointermove', function (e) {
      if (!ptrs[e.pointerId]) return
      var p = localPoint(e), prev = ptrs[e.pointerId]
      ptrs[e.pointerId] = p
      moved = Math.max(moved, Math.hypot(p.x - startX, p.y - startY))
      if (Object.keys(ptrs).length >= 2) {
        var d = dist()
        if (pinch && d) { var a = Object.keys(ptrs).map(function (i) { return ptrs[i] }); zoomAt((a[0].x + a[1].x) / 2, (a[0].y + a[1].y) / 2, k * d / pinch) }
        pinch = d
        return
      }
      if (panning) { x += p.x - prev.x; y += p.y - prev.y; apply() }
    })
    function up(e) {
      var wasTap = moved < 5 && Object.keys(ptrs).length === 1 && ptrs[e.pointerId]
      delete ptrs[e.pointerId]
      pinch = 0
      if (wasTap && kind === 'video' && e.type === 'pointerup') togglePlay()
    }
    on(stage, 'pointerup', up)
    on(stage, 'pointercancel', up)
    on(stage, 'dblclick', function (e) {
      if (kind === 'video') { toggleFull(); return }
      var p = localPoint(e)
      if (k > 1) resetZoom(); else zoomAt(p.x, p.y, 2.5)
    })

    // ---- fullscreen ----
    function isFull() { return document.fullscreenElement === root || document.webkitFullscreenElement === root }
    function toggleFull() {
      if (isFull()) { (document.exitFullscreen || document.webkitExitFullscreen || function () {}).call(document); return }
      var f = root.requestFullscreen || root.webkitRequestFullscreen
      if (f) f.call(root)
    }
    var fullBtn = btn('Fullscreen (f)', '⛶')
    on(fullBtn, 'click', toggleFull)
    on(document, 'fullscreenchange', function () { root.classList.toggle('mv-full', isFull()); apply() })
    on(document, 'webkitfullscreenchange', function () { root.classList.toggle('mv-full', isFull()); apply() })

    // ---- video controls ----
    var rafId = 0, seeking = false, togglePlay = function () {}
    var bar = el('div', 'mv-bar')
    if (kind === 'video') {
      var play = btn('Play (space)', '▶', 'mv-play')
      var seek = el('input', 'mv-seek', { type: 'range', min: '0', max: '1000', step: '1', value: '0', 'aria-label': 'Seek' })
      var time = el('span', 'mv-time')
      var speed = el('select', 'mv-speed', { 'aria-label': 'Playback speed' })
      SPEEDS.forEach(function (s) { var o = el('option', '', { value: String(s) }); o.textContent = s + '×'; if (s === 1) o.selected = true; speed.appendChild(o) })
      var mute = btn('Mute (m)', '🔊')
      var vol = el('input', 'mv-vol', { type: 'range', min: '0', max: '1', step: '0.05', value: '1', 'aria-label': 'Volume' })

      togglePlay = function () { if (media.paused) { var r = media.play(); if (r && r.catch) r.catch(function () {}) } else media.pause() }
      var paint = function () {
        var d = media.duration || 0, t = media.currentTime || 0
        if (!seeking) seek.value = d ? String(Math.round(t / d * 1000)) : '0'
        var buf = 0
        try { for (var i = 0; i < media.buffered.length; i++) if (media.buffered.start(i) <= t + 0.5) buf = Math.max(buf, media.buffered.end(i)) } catch (_) { /* no ranges yet */ }
        seek.style.setProperty('--mv-played', d ? (t / d * 100) + '%' : '0%')
        seek.style.setProperty('--mv-buf', d ? (Math.max(buf, t) / d * 100) + '%' : '0%')
        time.textContent = fmt(t) + ' / ' + fmt(d)
      }
      var loop = function () { paint(); rafId = media.paused ? 0 : requestAnimationFrame(loop) }
      on(play, 'click', togglePlay)
      on(media, 'play', function () {
        // only one video plays at a time
        var all = document.querySelectorAll('.mv video')
        for (var i = 0; i < all.length; i++) if (all[i] !== media) all[i].pause()
        play.textContent = '❚❚'; play.setAttribute('aria-label', 'Pause (space)')
        root.classList.add('mv-playing')
        if (!rafId) rafId = requestAnimationFrame(loop)
      })
      on(media, 'pause', function () { play.textContent = '▶'; play.setAttribute('aria-label', 'Play (space)'); root.classList.remove('mv-playing'); paint() })
      on(media, 'ended', function () { root.classList.remove('mv-playing') })
      ;['loadedmetadata', 'durationchange', 'progress', 'seeked', 'timeupdate'].forEach(function (ev) { on(media, ev, paint) })
      on(media, 'waiting', function () { root.classList.add('mv-wait') })
      ;['playing', 'canplay', 'seeked'].forEach(function (ev) { on(media, ev, function () { root.classList.remove('mv-wait') }) })
      on(media, 'error', function () {
        root.classList.remove('mv-wait')
        msg.textContent = 'This video cannot be played here (the format or codec may not be supported by this viewer). Try re-encoding it as MP4 (H.264/AAC) or WebM.'
        root.classList.add('mv-failed')
      })
      on(seek, 'input', function () { seeking = true; if (media.duration) media.currentTime = seek.value / 1000 * media.duration; paint() })
      on(seek, 'change', function () { seeking = false })
      on(speed, 'change', function () { media.playbackRate = Number(speed.value) })
      on(mute, 'click', function () { media.muted = !media.muted })
      on(vol, 'input', function () { media.volume = Number(vol.value); media.muted = media.volume === 0 })
      on(media, 'volumechange', function () { mute.textContent = media.muted || media.volume === 0 ? '🔇' : '🔊'; vol.value = media.muted ? '0' : String(media.volume) })
      bar.appendChild(play); bar.appendChild(seek); bar.appendChild(time); bar.appendChild(speed)
      bar.appendChild(mute); bar.appendChild(vol)
      var zg = el('span', 'mv-group'); zg.appendChild(zoomOut); zg.appendChild(zoomLabel); zg.appendChild(zoomIn)
      bar.appendChild(zg); bar.appendChild(fullBtn)
      root.appendChild(bar)
      paint()
    } else {
      var tools = el('div', 'mv-tools')
      tools.appendChild(zoomOut); tools.appendChild(zoomLabel); tools.appendChild(zoomIn); tools.appendChild(fullBtn)
      stage.appendChild(tools)
    }

    // ---- keyboard ----
    on(root, 'keydown', function (e) {
      var t = e.target && e.target.tagName
      if (t === 'INPUT' || t === 'SELECT' || e.altKey || e.ctrlKey || e.metaKey) return
      var key = e.key, handled = true
      if (key === '+' || key === '=') zoomBy(1.25)
      else if (key === '-' || key === '_') zoomBy(1 / 1.25)
      else if (key === '0') resetZoom()
      else if (key === 'f' || key === 'F') toggleFull()
      else if (kind === 'video' && (key === ' ' || key === 'k')) { if (t !== 'BUTTON') togglePlay(); else handled = false }
      else if (kind === 'video' && key === 'ArrowLeft') media.currentTime = Math.max(0, media.currentTime - 5)
      else if (kind === 'video' && key === 'ArrowRight') media.currentTime = Math.min(media.duration || 0, media.currentTime + 5)
      else if (kind === 'video' && key === 'ArrowUp') { media.muted = false; media.volume = Math.min(1, media.volume + 0.1) }
      else if (kind === 'video' && key === 'ArrowDown') media.volume = Math.max(0, media.volume - 0.1)
      else if (kind === 'video' && (key === 'm' || key === 'M')) media.muted = !media.muted
      else handled = false
      if (handled) e.preventDefault()
    })
    on(window, 'resize', apply)

    // ---- load the source (embedded data: URLs become blobs, which seek smoothly) ----
    var objectUrl = ''
    var dead = false
    function setSrc(u) { if (!dead) media.src = u }
    if (kind === 'video' && /^data:/.test(src) && window.fetch) {
      root.classList.add('mv-wait')
      fetch(src).then(function (r) { return r.blob() }).then(function (b) {
        if (dead) return
        objectUrl = URL.createObjectURL(b)
        setSrc(objectUrl)
      }).catch(function () { setSrc(src) })
    } else setSrc(src)
    apply()

    var api = {
      destroy: function () {
        dead = true
        if (rafId) cancelAnimationFrame(rafId)
        cleanups.forEach(function (f) { f() })
        if (kind === 'video') { try { media.pause() } catch (_) { /* detached */ } media.removeAttribute('src'); media.load() }
        if (objectUrl) URL.revokeObjectURL(objectUrl)
        root.textContent = ''
        delete root.__mv
      },
      media: media,
    }
    root.__mv = api
    return api
  }

  window.aicwMountAll = function (scope) {
    var list = (scope || document).querySelectorAll('.mv[data-src]')
    for (var i = 0; i < list.length; i++) window.aicwMountViewer(list[i])
  }
})()
