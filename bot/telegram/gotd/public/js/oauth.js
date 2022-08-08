function getEl(id) {
  return document.getElementById(id);
}

function ajax(url, data, callback, fallback) {
  try {
    var req = new XMLHttpRequest();
    req.open(data ? 'POST' : 'GET', url);
    req.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
    req.onreadystatechange = function() {
      if (req.readyState == XMLHttpRequest.DONE) {
        try {
          const res = JSON.parse(req.responseText);
          if (req.status === 200) {
            callback && callback(res, req);
          } else {
            throw res;
          }
        } catch (err) {
          fallback && fallback(err, req);
        }
      }
    };
    if (data) {
      req.setRequestHeader('Content-type', 'application/x-www-form-urlencoded');
      var postdata = [];
      for (var key in data) {
        postdata.push(encodeURIComponent(key) + '=' + encodeURIComponent(data[key]));
      }
      req.send(postdata.join('&'));
    } else {
      req.send();
    }
  } catch (err) {
    console.log(err);
  }
}

function cleanRE(value) {
  return value.replace(/[|\\{}()[\]^$+*?.]/g, "\\$&");
}

function inputFormatPhoneInit(init_country, init_phone_number, lang) {
  var CountriesList = window.CountriesList || [];
  var PrefixCountries = [], PrefixPatterns = [], prefix_map = {}, patterns_map = {}, i, j, k, c;
  for (i = 0; i < CountriesList.length; i++) {
    var country_data = CountriesList[i];
    if (!country_data.lname) {
      country_data.lname = country_data.name;
    }
    country_data.country_codes = [];
    for (c = 0; c < country_data.codes.length; c++) {
      var code_item = country_data.codes[c];
      var country_code = code_item.code;
      if (code_item.patterns) {
        for (j = 0; j < code_item.patterns.length; j++) {
          var pattern = code_item.patterns[j], prefix = '', ph_prefix = country_code, new_pattern = '';
          for (k = 0; k < pattern.length; k++) {
            if (pattern[k] >= '0' && pattern[k] <= '9') {
              prefix      += pattern[k];
              ph_prefix   += pattern[k];
              new_pattern += 'X';
            } else {
              new_pattern += pattern[k];
            }
          }
          if (!patterns_map[ph_prefix]) {
            PrefixPatterns.push([country_code, prefix, country_code + prefix, new_pattern]);
            patterns_map[ph_prefix] = true;
          }
        }
      }
      if (code_item.prefixes) {
        for (j = 0; j < code_item.prefixes.length; j++) {
          var prefix = code_item.prefixes[j], ph_prefix = country_code + prefix;
          if (!prefix_map[ph_prefix]) {
            PrefixCountries.push([country_code, prefix, country_code + prefix, country_data]);
            prefix_map[ph_prefix] = true;
          }
        }
      } else {
        if (!prefix_map[country_code]) {
          PrefixCountries.push([country_code, '', country_code, country_data]);
          prefix_map[country_code] = true;
        }
      }
      country_data.country_codes.push(country_code);
    }
  }
  try {
    var compare = new Intl.Collator(lang || 'en', {sensitivity: 'base'}).compare;
  } catch(e) {
    var compare = function(s1, s2) {
      s1 = s1.toLowerCase();
      s2 = s2.toLowerCase();
      return s1 < s2 ? -1 : (s1 > s2 ? 1 : 0);
    }
  }
  var compareListPrefix = function(p1, p2) {
    return (p1[2] < p2[2] ? 1 : (p1[2] > p2[2] ? -1 : 0));
  }
  var SortedCountriesList = [];
  for (var i = 0; i < CountriesList.length; i++) {
    if (!CountriesList[i].hidden) {
      var country_data = CountriesList[i];
      for (var j = 0; j < country_data.country_codes.length; j++) {
        var country_code = country_data.country_codes[j];
        SortedCountriesList.push({
          country_code: country_code,
          iso2: country_data.iso2,
          lname: country_data.lname,
          name: country_data.name,
          query_str: [
            country_data.iso2,
            country_data.name,
            country_data.lname,
            country_code
          ].join('\n')
        });
      }
    }
  }
  PrefixPatterns.sort(compareListPrefix);
  PrefixCountries.sort(compareListPrefix);
  SortedCountriesList.sort(function(p1, p2) {
    return compare(p1.lname, p2.lname);
  });
  var Keys = {
    BACKSPACE: 8,
    TAB: 9,
    RETURN: 13,
    ESC: 27,
    LEFT: 37,
    RIGHT: 39,
    UP: 38,
    DOWN: 40
  };

  var code_el = getEl('login-phone-code'),
    phone_el = getEl('login-phone'),
    placeholder_el = getEl('login-phone-placeholder'),
    country_wrap_el = getEl('login-country-wrap'),
    country_label_el = getEl('login-country-selected'),
    country_search_el = getEl('login-country-search'),
    search_results_el = getEl('login-country-search-results');

  function escapeHTML(html) {
    html = html || '';
    return html.replace(/&/g, '&amp;')
    .replace(/>/g, '&gt;')
    .replace(/</g, '&lt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');
  }

  var LastCountryCodeData = {};
  function getCountryDataByPrefix(value) {
    var data = null;
    if (data = LastCountryCodeData[value]) {
      return {prefix: value, iso2: data.iso2, lname: data.lname};
    }
    for (var i = 0; i < PrefixCountries.length; i++) {
      var country_code = PrefixCountries[i][0],
        prefix = PrefixCountries[i][2],
        data = PrefixCountries[i][3];
      if (value.indexOf(prefix) === 0) {
        return {prefix: country_code, iso2: data.iso2, lname: data.lname};
      }
    }
    return false;
  }

  function getPatternByPrefix(value) {
    for (var i = 0; i < PrefixPatterns.length; i++) {
      var pattern = PrefixPatterns[i][3], prefix = PrefixPatterns[i][2];
      if (value.indexOf(prefix) === 0) {
        return pattern;
      }
    }
    return false;
  }

  function getCountryDataByCountryCode(country_code) {
    for (var i = 0; i < CountriesList.length; i++) {
      var data = CountriesList[i];
      if (country_code == data.iso2) {
        return data;
      }
    }
    return false;
  }

  function mayBeCountryCode(value) {
    for (var i = 0; i < PrefixCountries.length; i++) {
      var country_code = PrefixCountries[i][0];
      if (country_code.indexOf(value) === 0) {
        return true;
      }
    }
    return false;
  }

  function onKeyDown(e) {
    if (e.target === phone_el &&
      (e.keyCode == Keys.LEFT ||
        e.keyCode == Keys.BACKSPACE) &&
      phone_el.selectionStart == phone_el.selectionEnd &&
      phone_el.selectionStart == 0) {
      code_el.focus();
      code_el.setSelectionRange(code_el.value.length, code_el.value.length);
    }
    else if (e.target === code_el &&
      e.keyCode == Keys.RIGHT &&
      code_el.selectionStart == code_el.selectionEnd &&
      code_el.selectionStart == code_el.value.length) {
      phone_el.focus();
      phone_el.setSelectionRange(0, 0);
    }
    else if (e.target === phone_el &&
      (e.keyCode == Keys.LEFT ||
        e.keyCode == Keys.BACKSPACE) &&
      phone_el.selectionStart == phone_el.selectionEnd &&
      phone_el.value.substr(phone_el.selectionStart - 1, 1) == ' ') {
      phone_el.setSelectionRange(phone_el.selectionStart - 1, phone_el.selectionStart - 1);
    }
    else if (e.target === phone_el &&
      e.keyCode == Keys.RIGHT &&
      phone_el.selectionStart == phone_el.selectionEnd &&
      phone_el.value.substr(phone_el.selectionStart, 1) == ' ') {
      phone_el.setSelectionRange(phone_el.selectionStart + 1, phone_el.selectionStart + 1);
    }
  }
  function onInput(e) {
    if (e && (e.keyCode < 48 || e.keyCode > 57)) {
      return false;
    }
    var code = code_el.value;
    var number = phone_el.value;
    var value = (code + number).substr(0, 24);
    if (document.activeElement === code_el) {
      var selectionStart = code_el.selectionStart;
    } else {
      var selectionStart = code_el.value.length + phone_el.selectionStart;
    }
    var prefix = value.substr(0, selectionStart);
    value = value.replace(/[^0-9]/g, '');
    prefix = prefix.replace(/[^0-9]/g, '');
    var prefix_len = prefix.length;
    var found_data = getCountryDataByPrefix(value);
    var newSelectionStart = 1 + prefix_len;
    var isPrefixFull = false;
    var pk = 0;
    if (found_data) {
      isPrefixFull = true;
      var prefix = found_data.prefix, format = getPatternByPrefix(value) || '';
      var suffix = value.substr(prefix.length);
      var new_code = '+' + prefix, new_value = ''
      var new_placeholder = new_value;
      pk += prefix.length;
      for (var j = 0, k = 0; j < format.length; j++) {
        if (format[j] == 'X') {
          new_value += suffix[k] || '';
          new_placeholder += suffix[k] || '−';
          k++; pk++;
        } else {
          new_value += (k < suffix.length) ? format[j] : '';
          new_placeholder += format[j];
          if (pk < prefix_len) newSelectionStart++;
        }
      }
      if (k < suffix.length) {
        new_value += suffix.substr(k);
      }
      country_label_el.innerHTML = found_data.lname;
      country_label_el.classList.add('is-dirty');
      country_search_el.value = searchLastValue = found_data.lname;
    } else {
      var new_code = '+', new_value = '';
      if (mayBeCountryCode(value)) {
        new_code += value;
      } else {
        isPrefixFull = true;
        new_value += value;
      }
      var new_placeholder = new_value;
      country_label_el.innerHTML = country_label_el.getAttribute('data-placeholder');
      country_label_el.classList.remove('is-dirty');
      country_search_el.value = searchLastValue = '';
    }
    if (!new_placeholder.length && !new_value.length) {
      new_placeholder = placeholder_el.getAttribute('data-placeholder') || '';
    }
    placeholder_el.innerHTML = new_placeholder;
    code_el.value = new_code;
    phone_el.value = new_value;
    if (newSelectionStart > new_code.length ||
      isPrefixFull && newSelectionStart == new_code.length) {
      newSelectionStart -= new_code.length;
      var focused_el = phone_el;
    } else {
      var focused_el = code_el;
    }
    focused_el.focus();
    focused_el.setSelectionRange(newSelectionStart, newSelectionStart);
    setTimeout(function() {
      focused_el.setSelectionRange(newSelectionStart, newSelectionStart);
    }, 0);
    code_el.parentNode.classList.remove('is-invalid');
    phone_el.parentNode.classList.remove('is-invalid');
    searchClose(true);
  }

  function adjustScroll(el) {
    var scrollTop   = search_results_el.scrollTop,
      itemTop     = el.offsetTop,
      itemBottom  = itemTop + el.offsetHeight,
      contHeight  = search_results_el.offsetHeight;

    if (itemTop < scrollTop) {
      search_results_el.scrollTop = itemTop;
    } else if (itemBottom > scrollTop + contHeight) {
      search_results_el.scrollTop = itemBottom - contHeight;
    }
  }

  function onSearchDocumentKeyDown(e) {
    if (e.keyCode == Keys.ESC) {
      e.preventDefault();
      searchClose();
    }
    else if (e.keyCode == Keys.UP) {
      e.preventDefault();
      if (searchSelectedEl && searchSelectedEl.previousSibling) {
        searchItemOver(searchSelectedEl.previousSibling, true);
      }
    }
    else if (e.keyCode == Keys.DOWN) {
      e.preventDefault();
      if (searchSelectedEl && searchSelectedEl.nextSibling) {
        searchItemOver(searchSelectedEl.nextSibling, true);
      }
    }
    else if (e.keyCode == Keys.TAB) {
      e.preventDefault();
      searchClose();
    }
    else if (e.keyCode == Keys.RETURN) {
      e.preventDefault();
      searchItemSelect(searchSelectedEl);
    }
  }

  function onClickOutside(e) {
    searchClose();
  }

  function onSearchClick(e) {
    e.stopPropagation();
  }

  function onSearchKeyDown(e) {
    setTimeout(onSearchInput, 0, e);
  }

  function onSearchInput(e) {
    if (searchLastValue != country_search_el.value) {
      searchLastValue = country_search_el.value;
      updateSearchResults(country_search_el.value);
    }
  }

  var searchSelectedEl, searchLastValue;

  function updateSearchResults(query) {
    query = (query || '');
    var bre = '(^|[\\s,.:;"\'\\-])';
    var re = new RegExp(bre + cleanRE(query || ''), 'i');
    var html = '', found = false;
    for (var i = 0; i < SortedCountriesList.length; i++) {
      var data = SortedCountriesList[i];
      if (!query ||
        re.test(data.query_str) ||
        query[0] == '+' && (new RegExp(bre + cleanRE(query.substr(1)), 'i')).test(data.country_code)) {
        found = true;
        html += '<div class="login_country_search_result" data-code="' + data.iso2 + '" data-prefix="' + data.country_code + '"><span dir="auto">' + data.lname + '</span><span dir="auto" class="prefix">+' + data.country_code + '</span></div>';
      }
    }
    if (!found) {
      var noresult = search_results_el.getAttribute('data-noresult');
      html = '<div class="login_country_search_noresult">' + escapeHTML(noresult) + '</div>';
    }
    search_results_el.innerHTML = html;
    searchItemOver(found ? search_results_el.children[0] : null, true);
  }

  function onSearchItemOver(e) {
    var el = e.target;
    while (el && el.classList) {
      if (el.classList.contains('login_country_search_result')) {
        searchItemOver(el);
        return;
      }
      el = el.parentNode;
    }
  }

  function onSearchItemClick(e) {
    e.stopPropagation();
    var el = e.target;
    while (el && el.classList) {
      if (el.classList.contains('login_country_search_result')) {
        searchItemSelect(el);
        return;
      }
      el = el.parentNode;
    }
  }

  function searchItemOver(el, adjust) {
    if (searchSelectedEl) {
      searchSelectedEl.classList.remove('selected');
    }
    if (el) {
      searchSelectedEl = el;
      el.classList.add('selected');
      adjust && adjustScroll(el);
    } else {
      searchSelectedEl = null;
    }
  }

  function searchItemSelect(el) {
    if (!el) {
      searchClose();
      return;
    }
    var country_code = el.getAttribute('data-code');
    var prefix = el.getAttribute('data-prefix');
    var data = getCountryDataByCountryCode(country_code);
    if (data) {
      var postfix = phone_el.value;
      LastCountryCodeData[prefix] = data;
      var new_value = prefix + postfix.replace(/[^0-9]/g, '');
      var new_data = getCountryDataByPrefix(new_value);
      if (new_data.iso2 != country_code) {
        phone_el.value = '';
      }
      code_el.value = prefix;
    }
    searchClose();
  }

  function onSearchOpen(e) {
    e.stopPropagation();
    country_wrap_el.classList.add('opened');
    country_search_el.value = searchLastValue = '';
    setTimeout(function() {
      country_search_el.focus();
      country_search_el.setSelectionRange(0, country_search_el.value.length);
    }, 50);
    updateSearchResults(country_search_el.value);
    document.addEventListener('keydown', onSearchDocumentKeyDown);
    document.addEventListener('click', onClickOutside);
  }

  function searchClose(no_event) {
    country_wrap_el.classList.remove('opened');
    document.removeEventListener('keydown', onSearchDocumentKeyDown);
    document.removeEventListener('click', onClickOutside);
    phone_el.setSelectionRange(phone_el.value.length, phone_el.value.length);
    if (!no_event) {
      onInput();
    }
  }

  code_el.addEventListener('input', onInput);
  code_el.addEventListener('keydown', onKeyDown);
  phone_el.addEventListener('input', onInput);
  phone_el.addEventListener('keydown', onKeyDown);

  country_label_el.addEventListener('click', onSearchOpen);
  country_search_el.addEventListener('click', onSearchClick);
  country_search_el.addEventListener('input', onSearchInput);
  country_search_el.addEventListener('keydown', onSearchKeyDown);
  search_results_el.addEventListener('mouseover', onSearchItemOver);
  search_results_el.addEventListener('click', onSearchItemClick);

  var init_prefix = '+';
  if (init_phone_number) {
    init_prefix += init_phone_number;
  }
  else if (init_country) {
    var data = getCountryDataByCountryCode(init_country);
    if (data) {
      if (data && data.codes.length == 1) {
        init_prefix += data.codes[0].code;
      }
    }
  }
  code_el.value = init_prefix;
  phone_el.focus();
  onInput();
}

function redraw(el) {
  el.offsetTop + 1;
}

function initRipple() {
  if (!document.querySelectorAll) return;
  var rippleTextFields = document.querySelectorAll('.textfield-item input.form-control');
  for (var i = 0; i < rippleTextFields.length; i++) {
    (function(rippleTextField) {
      function onTextRippleStart(e) {
        if (document.activeElement === rippleTextField) return;
        var rect = rippleTextField.getBoundingClientRect();
        if (e.type == 'touchstart') {
          var clientX = e.targetTouches[0].clientX;
        } else {
          var clientX = e.clientX;
        }
        var ripple = rippleTextField.parentNode.querySelector('.textfield-item-underline');
        var rippleX = (clientX - rect.left) / rippleTextField.offsetWidth * 100;
        ripple.style.transition = 'none';
        redraw(ripple);
        ripple.style.left = rippleX + '%';
        ripple.style.right = (100 - rippleX) + '%';
        redraw(ripple);
        ripple.style.left = '';
        ripple.style.right = '';
        ripple.style.transition = '';
      }
      rippleTextField.addEventListener('mousedown', onTextRippleStart);
      rippleTextField.addEventListener('touchstart', onTextRippleStart);
    })(rippleTextFields[i]);
  }
  var rippleHandlers = document.querySelectorAll('.ripple-handler');
  for (var i = 0; i < rippleHandlers.length; i++) {
    (function(rippleHandler) {
      function onRippleStart(e) {
        var rippleMask = rippleHandler.querySelector('.ripple-mask');
        if (!rippleMask) return;
        var rect = rippleMask.getBoundingClientRect();
        if (e.type == 'touchstart') {
          var clientX = e.targetTouches[0].clientX;
          var clientY = e.targetTouches[0].clientY;
        } else {
          var clientX = e.clientX;
          var clientY = e.clientY;
        }
        var rippleX = (clientX - rect.left) - rippleMask.offsetWidth / 2;
        var rippleY = (clientY - rect.top) - rippleMask.offsetHeight / 2;
        var ripple = rippleHandler.querySelector('.ripple');
        ripple.style.transition = 'none';
        redraw(ripple);
        ripple.style.transform = 'translate3d(' + rippleX + 'px, ' + rippleY + 'px, 0) scale3d(0.2, 0.2, 1)';
        ripple.style.opacity = 1;
        redraw(ripple);
        ripple.style.transform = 'translate3d(' + rippleX + 'px, ' + rippleY + 'px, 0) scale3d(1, 1, 1)';
        ripple.style.transition = '';

        function onRippleEnd(e) {
          ripple.style.transitionDuration = '.2s';
          ripple.style.opacity = 0;
          document.removeEventListener('mouseup', onRippleEnd);
          document.removeEventListener('touchend', onRippleEnd);
          document.removeEventListener('touchcancel', onRippleEnd);
        }
        document.addEventListener('mouseup', onRippleEnd);
        document.addEventListener('touchend', onRippleEnd);
        document.addEventListener('touchcancel', onRippleEnd);
      }
      rippleHandler.addEventListener('mousedown', onRippleStart);
      rippleHandler.addEventListener('touchstart', onRippleStart);
    })(rippleHandlers[i]);
  }
}
