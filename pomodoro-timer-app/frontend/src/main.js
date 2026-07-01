// ==================== State Management ====================
const State = { IDLE: 'idle', RUNNING: 'running', PAUSED: 'paused' };
const Mode = { WORK: 'work', SHORT_BREAK: 'shortBreak', LONG_BREAK: 'longBreak' };

const state = {
  mode: Mode.WORK,
  status: State.IDLE,
  timeRemaining: 25 * 60,
  totalTime: 25 * 60,
  pomodoroCount: 0,
  intervalId: null,
  targetTimestamp: null
};

// ==================== DOM ====================
const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

const dom = {};

function cacheDom() {
  dom.timerTime      = $('#timerTime');
  dom.modeLabel      = $('#modeLabel');
  dom.ringProgress   = $('#ringProgress');
  dom.pomodoroCount  = $('#pomodoroCount');
  dom.startPauseBtn  = $('#startPauseBtn');
  dom.startPauseText = $('#startPauseText');
  dom.playPauseIcon  = $('#playPauseIcon');
  dom.resetBtn       = $('#resetBtn');
  dom.modeTabs       = $('#modeTabs');
  dom.settingsToggle = $('#settingsToggle');
  dom.settingsPanel  = $('#settingsPanel');
  dom.workDuration   = $('#workDuration');
  dom.shortBreakDur  = $('#shortBreakDuration');
  dom.longBreakDur   = $('#longBreakDuration');
  dom.soundToggle    = $('#soundToggle');
}

// ==================== Settings ====================
const STORAGE_KEY = 'pomodoro-settings';
const defaultSettings = { work: 25, shortBreak: 5, longBreak: 15, sound: true };

let settings = loadSettings();

function loadSettings() {
  try {
    const s = JSON.parse(localStorage.getItem(STORAGE_KEY));
    return s ? { ...defaultSettings, ...s } : { ...defaultSettings };
  } catch { return { ...defaultSettings }; }
}

function saveSettings() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}

function syncSettingsUI() {
  dom.workDuration.value  = settings.work;
  dom.shortBreakDur.value = settings.shortBreak;
  dom.longBreakDur.value  = settings.longBreak;
  dom.soundToggle.checked = settings.sound;
}

function onSettingChange() {
  settings.work       = Math.max(1, Math.min(120, parseInt(dom.workDuration.value) || 25));
  settings.shortBreak = Math.max(1, Math.min(60, parseInt(dom.shortBreakDur.value) || 5));
  settings.longBreak  = Math.max(1, Math.min(60, parseInt(dom.longBreakDur.value) || 15));
  settings.sound      = dom.soundToggle.checked;
  saveSettings();
  if (state.status === State.IDLE) {
    const dur = getDurationForMode(state.mode);
    state.timeRemaining = dur;
    state.totalTime = dur;
    updateDisplay();
  }
}

function getDurationForMode(mode) {
  switch (mode) {
    case Mode.WORK:        return settings.work * 60;
    case Mode.SHORT_BREAK: return settings.shortBreak * 60;
    case Mode.LONG_BREAK:  return settings.longBreak * 60;
    default:               return settings.work * 60;
  }
}

// ==================== Audio ====================
let audioCtx = null;

function playNotification() {
  if (!settings.sound) return;
  if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();
  const freqs = [523.25, 659.25, 783.99];
  freqs.forEach((freq, i) => {
    const osc  = audioCtx.createOscillator();
    const gain = audioCtx.createGain();
    osc.type = 'sine';
    osc.frequency.value = freq;
    const t = audioCtx.currentTime + i * 0.35;
    gain.gain.setValueAtTime(0.3, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.2);
    osc.connect(gain);
    gain.connect(audioCtx.destination);
    osc.start(t);
    osc.stop(t + 0.25);
  });
}

// ==================== SVG Ring ====================
const RING_RADIUS = 90;
const CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS;

function initRing() {
  dom.ringProgress.style.strokeDasharray  = CIRCUMFERENCE;
  dom.ringProgress.style.strokeDashoffset = 0;
}

function updateRing() {
  const progress = state.totalTime > 0 ? state.timeRemaining / state.totalTime : 0;
  dom.ringProgress.style.strokeDashoffset = CIRCUMFERENCE * (1 - progress);
}

// ==================== Display ====================
function formatTime(s) {
  return `${Math.floor(s/60).toString().padStart(2,'0')}:${(s%60).toString().padStart(2,'0')}`;
}

function getModeLabel(m) {
  return m === Mode.WORK ? 'Work' : m === Mode.SHORT_BREAK ? 'Short Break' : 'Long Break';
}

function getModeColor(m) {
  return m === Mode.WORK ? 'var(--bg-work)' : m === Mode.SHORT_BREAK ? 'var(--bg-short)' : 'var(--bg-long)';
}

function updateDisplay() {
  dom.timerTime.textContent    = formatTime(state.timeRemaining);
  dom.modeLabel.textContent    = getModeLabel(state.mode);
  dom.pomodoroCount.textContent = state.pomodoroCount;
  document.body.style.backgroundColor = getModeColor(state.mode);
  updateRing();

  dom.timerTime.classList.remove('running', 'completed');
  if (state.status === State.RUNNING) dom.timerTime.classList.add('running');

  $$('.mode-tab').forEach(t => t.classList.toggle('active', t.dataset.mode === state.mode));
  document.title = `${formatTime(state.timeRemaining)} - ${getModeLabel(state.mode)} | Pomodoro`;
}

function updateControls() {
  const running = state.status === State.RUNNING;
  dom.playPauseIcon.textContent  = running ? '⏸' : '▶';
  dom.startPauseText.textContent = running ? 'Pause' : 'Start';
}

// ==================== Timer ====================
function tick() {
  const remaining = Math.max(0, Math.ceil((state.targetTimestamp - Date.now()) / 1000));
  state.timeRemaining = remaining;
  updateDisplay();
  if (remaining <= 0) onSessionComplete();
}

function startTimer() {
  if (state.status === State.RUNNING) return;
  state.status = State.RUNNING;
  state.targetTimestamp = Date.now() + state.timeRemaining * 1000;
  state.intervalId = setInterval(tick, 250);
  updateControls();
  updateDisplay();
}

function pauseTimer() {
  if (state.status !== State.RUNNING) return;
  state.status = State.PAUSED;
  clearInterval(state.intervalId);
  state.intervalId = null;
  state.timeRemaining = Math.max(0, Math.ceil((state.targetTimestamp - Date.now()) / 1000));
  updateControls();
  updateDisplay();
}

function resetTimer() {
  clearInterval(state.intervalId);
  state.intervalId = null;
  state.status = State.IDLE;
  const dur = getDurationForMode(state.mode);
  state.timeRemaining = dur;
  state.totalTime = dur;
  updateControls();
  updateDisplay();
}

// ==================== Session Transition ====================
function onSessionComplete() {
  clearInterval(state.intervalId);
  state.intervalId = null;
  state.status = State.IDLE;
  playNotification();

  dom.timerTime.classList.remove('running');
  dom.timerTime.classList.add('completed');
  setTimeout(() => dom.timerTime.classList.remove('completed'), 3000);

  if (state.mode === Mode.WORK) {
    state.pomodoroCount++;
    if (state.pomodoroCount >= 4) {
      state.pomodoroCount = 0;
      switchMode(Mode.LONG_BREAK, true);
    } else {
      switchMode(Mode.SHORT_BREAK, true);
    }
  } else {
    switchMode(Mode.WORK, true);
  }
  updateControls();
}

function switchMode(mode, autoStart = false) {
  state.mode = mode;
  const dur = getDurationForMode(mode);
  state.timeRemaining = dur;
  state.totalTime = dur;
  updateDisplay();
  if (autoStart) setTimeout(startTimer, 500);
}

// ==================== Events ====================
function bindEvents() {
  dom.startPauseBtn.addEventListener('click', () => {
    state.status === State.RUNNING ? pauseTimer() : startTimer();
  });

  dom.resetBtn.addEventListener('click', resetTimer);

  dom.modeTabs.addEventListener('click', (e) => {
    const tab = e.target.closest('.mode-tab');
    if (!tab) return;
    if (state.status === State.RUNNING) {
      clearInterval(state.intervalId);
      state.intervalId = null;
      state.status = State.IDLE;
    }
    switchMode(tab.dataset.mode, false);
    updateControls();
  });

  dom.settingsToggle.addEventListener('click', () => {
    const open = dom.settingsPanel.classList.toggle('open');
    dom.settingsToggle.classList.toggle('open', open);
  });

  [dom.workDuration, dom.shortBreakDur, dom.longBreakDur].forEach(el => {
    el.addEventListener('change', onSettingChange);
    el.addEventListener('input', onSettingChange);
  });
  dom.soundToggle.addEventListener('change', onSettingChange);

  document.addEventListener('keydown', (e) => {
    if (e.target.tagName === 'INPUT') return;
    switch (e.code) {
      case 'Space':
        e.preventDefault();
        state.status === State.RUNNING ? pauseTimer() : startTimer();
        break;
      case 'KeyR': resetTimer(); break;
      case 'Digit1': switchMode(Mode.WORK); updateControls(); break;
      case 'Digit2': switchMode(Mode.SHORT_BREAK); updateControls(); break;
      case 'Digit3': switchMode(Mode.LONG_BREAK); updateControls(); break;
    }
  });
}

// ==================== Init ====================
function init() {
  cacheDom();
  initRing();
  syncSettingsUI();
  bindEvents();

  const dur = getDurationForMode(state.mode);
  state.timeRemaining = dur;
  state.totalTime = dur;
  updateControls();
  updateDisplay();
}

// Module scripts are deferred — DOM is ready by execution time
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
