const root = document.documentElement;
const screenSelect = document.querySelector('#screen-select');
const accountTrigger = document.querySelector('#account-trigger');
const accountMenu = document.querySelector('#account-menu');
const screens = [...document.querySelectorAll('[data-screen]')];
const screenNames = {
  live: 'Directo', movies: 'Películas', series: 'Series', detail: 'Ficha de título',
  favorites: 'Mi lista', ace: 'Ace Stream', source: 'Conectar fuente', login: 'Acceso',
  'vod-player': 'Reproductor VOD', 'live-player': 'Reproductor en directo', kit: 'Sistema visual'
};

document.querySelectorAll('.app-sidebar .nav-item[aria-label], .top-nav-compact .nav-item[aria-label]').forEach((item) => {
  item.title = item.getAttribute('aria-label');
});

function showScreen(name) {
  if (!screenNames[name]) return;
  closeAccountMenu(false);
  root.dataset.screen = name;
  screenSelect.value = name;
  screens.forEach((screen) => { screen.hidden = screen.dataset.screen !== name; });
  document.querySelector('#crumb-label').textContent = screenNames[name];
  document.querySelectorAll('[data-screen-link]').forEach((item) => {
    const active = item.dataset.screenLink === name;
    item.classList.toggle('active', active && item.classList.contains('nav-item'));
    if (item.matches('.mobile-bottom-nav button')) item.classList.toggle('is-current', active);
  });
}

function closeAccountMenu(returnFocus) {
  if (!accountMenu || accountMenu.hidden) return;
  accountMenu.hidden = true;
  accountTrigger.setAttribute('aria-expanded', 'false');
  if (returnFocus) accountTrigger.focus();
}

document.querySelectorAll('[data-direction-button]').forEach((button) => {
  button.addEventListener('click', () => {
    root.dataset.direction = button.dataset.directionButton;
    document.querySelectorAll('[data-direction-button]').forEach((candidate) => {
      const selected = candidate === button;
      candidate.classList.toggle('is-selected', selected);
      candidate.setAttribute('aria-pressed', String(selected));
    });
  });
});

document.querySelectorAll('[data-size-button]').forEach((button) => {
  button.addEventListener('click', () => {
    root.dataset.size = button.dataset.sizeButton;
    document.querySelectorAll('[data-size-button]').forEach((candidate) => {
      const selected = candidate === button;
      candidate.classList.toggle('is-selected', selected);
      candidate.setAttribute('aria-pressed', String(selected));
    });
  });
});

screenSelect.addEventListener('change', (event) => showScreen(event.target.value));
document.querySelectorAll('[data-screen-link]').forEach((item) => {
  item.addEventListener('click', (event) => {
    event.preventDefault();
    showScreen(item.dataset.screenLink);
  });
});

accountTrigger.addEventListener('click', () => {
  const opening = accountMenu.hidden;
  accountMenu.hidden = !opening;
  accountTrigger.setAttribute('aria-expanded', String(opening));
  if (opening) accountMenu.querySelector('button').focus();
});
document.addEventListener('click', (event) => {
  if (!event.target.closest('.account-menu-wrap')) closeAccountMenu(false);
});

document.querySelector('#ace-form').addEventListener('submit', (event) => {
  event.preventDefault();
  event.currentTarget.querySelector('.form-feedback').textContent = 'Vista de demostración: no se ha abierto ningún reproductor.';
});
document.querySelector('#source-form').addEventListener('submit', (event) => {
  event.preventDefault();
  event.currentTarget.querySelector('.form-feedback').textContent = 'Prototipo visual: la fuente no se envía ni se guarda.';
});
document.querySelector('#login-form').addEventListener('submit', (event) => {
  event.preventDefault();
  showScreen('live');
});

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && !accountMenu.hidden) {
    closeAccountMenu(true);
    return;
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    showScreen('movies');
    document.querySelector('[data-screen="movies"] input[type="search"]').focus();
  }
  if (event.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement.tagName)) {
    event.preventDefault();
    showScreen('movies');
    document.querySelector('[data-screen="movies"] input[type="search"]').focus();
  }
});

showScreen('live');
