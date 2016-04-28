(function() {
  function load(path, selection) {
    var offset = 0;

    var req = new XMLHttpRequest();
    req.onreadystatechange = handleStateChange;
    req.open('GET', path);
    req.send();

    function handleStateChange() {
      var text = req.responseText;
      var length = text.length;
      if (selection) {
        if (offset <= selection[0] && offset+length > selection[0]) {
          appendChunk(text.substring(offset, selection[0]));
          offset = selection[0];
        }
        if (offset >= selection[0] && offset < selection[1]) {
          if (offset + length >= selection[1]) {
            appendChunk(text.substring(selection[0], selection[1]), 'selection');
            offset = selection[1];
          }
          else {
            offset = length;
          }
        }
      }
      if (offset < length) {
        appendChunk(text.substring(offset, length));
      }

      if (req.readyState === 4) {
        if (selection) {
          var span = $('.selection');
          if (span.size()) {
            $('body').scrollTop(span.offset().top);
          }
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
    var left = getOffset(selection.baseNode) + selection.baseOffset;
    var right = getOffset(selection.extentNode) + selection.extentOffset;
    location.hash = left === right ? '' : Math.min(left, right) + '-' + Math.max(left, right);
  }

  function getOffset(node) {
    return $(node.parentElement).prevAll().toArray().reduce(function(length, node) {
      return length + node.innerText.length;
    }, 0);
  }

  window.orl = {
    load: load
  };
})();