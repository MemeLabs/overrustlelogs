(function () {
  var HEADER_HEIGHT = 64;

  function load(path, selection) {
    var offset = 0;
    var search = null;
    var searchParams = getSearchParams();
    if (searchParams.search) {
      search = searchParams.search;
    }

    var req = new XMLHttpRequest();
    req.onreadystatechange = handleStateChange;
    req.open('GET', path);
    req.send();

    function handleStateChange() {
      var text = req.responseText;
      var length = text.length;
      if (selection) {
        if (offset < selection[0] && length >= selection[0]) {
          appendChunk(text.substring(offset, selection[0]));
          offset = selection[0];
        }
        if (offset >= selection[0] && offset < selection[1]) {
          if (length >= selection[1]) {
            appendChunk(text.substring(selection[0], selection[1]), 'selection');
            offset = selection[1];
          } else {
            offset = length;
          }
        }
      }
      if (offset < length) {
        appendChunk(text.substring(offset, length));
        offset = length;
      }

      if (req.readyState === 4) {
        if (selection) {
          $(".selection")[0].scrollIntoView();
          window.scrollBy(0, - HEADER_HEIGHT);
          // var span = $('.selection');
          // if (span.length) {
          //   // $("html, body").animate({ scrollTop: $(".selection").first().offset().top - HEADER_HEIGHT }, 10);
          //   span.scrollIntoView();
          // }
        }
        else if (search) {
          find(search);
        }

        $('.text').on('mouseup', updateHash);
      }
    }
  }

  function appendChunk(chunk, className) {
    $('<span />').attr('class', className || null).text(chunk).appendTo('.text');
  }

  function updateHash() {
    var selection = document.getSelection();
    var left = getOffset(selection.anchorNode) + selection.anchorOffset;
    var right = getOffset(selection.focusNode) + selection.focusOffset;
    var hash = left === right ? '' : '#' + Math.min(left, right) + '-' + Math.max(left, right);
    var path = window.location.pathname;
    if (window.location.search !== '') {
      path += window.location.search;
    }
    history.replaceState('', document.title, path + hash);
  }

  function getOffset(node) {
    return $(node.parentElement).prevAll().toArray().reduce(function (length, node) {
      return length + node.textContent.length;
    }, 0);
  }

  // https://stackoverflow.com/a/47444595
  function getSearchParams() {
    location.search
      .slice(1)
      .split('&')
      .map(p => p.split('='))
      .reduce((obj, pair) => {
        const [key, value] = pair.map(decodeURIComponent);
        return ({ ...obj, [key]: value })
      }, {})
      ;
  }

  window.orl = {
    load: load
  };
})();
