// {{define "JS"}}

// Query highlighting
function highlightQuery() {
  var resultsList = document.getElementById("results");
  if (!resultsList || resultsList.childNodes.length == 0) return; // nothing to highlight
  var q = document.getElementById("q").value;
  if (q) {
    q.split(/[^\w]+/).forEach(function(p) {
      p=p.trim();
      if (!p) return;
      $("pre.sourcegraph-code span:contains('" + p.replace(/[^\w.-]/g, "") + "')").css("background-color", "yellow");
    });
  }
}
document.addEventListener("DOMContentLoaded", highlightQuery);

// PJAX
$(document).on("submit", "form[data-pjax]", function(event) {
  $.pjax.submit(event, {container: $("[data-pjax-container]")});
  event.preventDefault();
});
document.addEventListener("pjax:end", function() {
  highlightQuery();
  $("[autofocus]").focus();
});


$.pjax.defaults.timeout = 60000; // Time out after 60 seconds.
$(document).on("pjax:send", function(ev) {
  $("#fake-results-container").show().addClass("show");
  $("#results-container").hide();
});
$(document).on("pjax:complete", function(ev) {
  $("#fake-results-container").hide().removeClass("show");
  $("#results-container").show();
});

// Hack to allow document.addEventListener("pjax:end", ...) listeners
// to be triggered. Without this, you need to use jQuery to listen to
// the event.
$(document).on("pjax:end", function(ev) {
  if (!ev.originalEvent) document.dispatchEvent(new Event("pjax:end"));
});


// Use PJAX for example queries.
document.addEventListener("DOMContentLoaded", function() {
  var exampleQueries = document.getElementById("example-queries");
  $(exampleQueries).pjax("a", "[data-pjax-container]");

  // Update the query field with the example query after a click.
  exampleQueries.addEventListener("click", function(ev) {
    var $a = ev.target;
    var $q = document.getElementById("q");
    $q.value = $a.dataset.query;
    $q.focus();
  });
});

// Automatically start searching after typing.
document.addEventListener("DOMContentLoaded", function() {
  var $q = document.getElementById("q");
  var lastValue = $q.value.trim();

  var handleKeyInput = debounce(function(ev) {
    if ($q.value.trim() == lastValue) return;
    lastValue = $q.value.trim();
    var fakeEvent = {currentTarget: $q.form, type: "submit", preventDefault: function() {}};
    $.pjax.submit(fakeEvent, {blurInput: false, container: $("[data-pjax-container]"), replace: true});
  }, 500, false);

  $($q).on("keypress", handleKeyInput);
  $($q).on("keyup", function(event) {
    // DELETE or BACKSPACE key.
    if (event.which == 8 || event.which == 46) handleKeyInput(event);
  });
});


function debounce(callback, delay) {
  var self = this, timeout, _arguments;
  return function() {
    _arguments = Array.prototype.slice.call(arguments, 0),
    timeout = clearTimeout(timeout, _arguments),
    timeout = setTimeout(function() {
      callback.apply(self, _arguments);
      timeout = 0;
    }, delay);
    return this;
  };
};


// {{end}}
