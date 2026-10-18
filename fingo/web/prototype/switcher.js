/*
 * Prototype variant switcher. THROWAWAY.
 *
 * Reads ?variant=A|B|C from the URL, shows the matching [data-variant] block,
 * and puts a floating bar at the bottom to cycle. Arrow keys work too, except
 * while a text field has focus.
 *
 * There is no server here on purpose: these are files you open in a browser.
 * location.search works under file://, so ?variant= survives a double-click.
 */
(function () {
  var nodes = Array.prototype.slice.call(document.querySelectorAll('[data-variant]'));
  if (!nodes.length) return;

  var variants = nodes.map(function (el) {
    return {
      key: el.getAttribute('data-variant'),
      name: el.getAttribute('data-variant-name') || el.getAttribute('data-variant'),
      el: el
    };
  });

  var wanted = new URLSearchParams(location.search).get('variant') || 'A';
  var index = variants.findIndex(function (v) { return v.key === wanted; });
  if (index < 0) index = 0;

  var bar = document.getElementById('protoSwitch');
  if (bar) {
    bar.innerHTML =
      '<button class="proto-prev" type="button" aria-label="Previous variant">&#8249;</button>' +
      '<span class="proto-label"></span>' +
      '<button class="proto-next" type="button" aria-label="Next variant">&#8250;</button>';
  }

  function show(next) {
    index = ((next % variants.length) + variants.length) % variants.length;

    variants.forEach(function (v, n) { v.el.hidden = n !== index; });

    var label = bar && bar.querySelector('.proto-label');
    if (label) label.textContent = variants[index].key + ' (' + variants[index].name + ')';

    var url = new URL(location.href);
    url.searchParams.set('variant', variants[index].key);
    history.replaceState(null, '', url);
  }

  document.addEventListener('click', function (event) {
    if (event.target.closest('.proto-prev')) show(index - 1);
    if (event.target.closest('.proto-next')) show(index + 1);
  });

  document.addEventListener('keydown', function (event) {
    var t = event.target;
    if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return;
    if (event.key === 'ArrowLeft') show(index - 1);
    if (event.key === 'ArrowRight') show(index + 1);
  });

  show(index);
})();
