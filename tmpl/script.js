// {{define "JS"}}

// Query highlighting
function highlightQuery() {
  var resultsList = document.getElementById("results");
  if (!resultsList || resultsList.childNodes.length == 0) return; // nothing to highlight
  var q = document.getElementById("q").value;
  if (q) {
    q.toLowerCase().split(/[^\w]+/).forEach(function(p) {
      p=p.trim();
      if (!p) return;
      var spans = resultsList.querySelectorAll("pre.sourcegraph-code span");
      for (var i = 0; i < spans.length; i++) {
        if (spans[i].innerText.toLowerCase().indexOf(p) !== -1) {
          spans[i].style.backgroundColor = "yellow";
        }
      }
    });
  }
}
document.addEventListener("DOMContentLoaded", highlightQuery);

function search(ev, q, pjaxOpt) {
  if (q) {
    $("#welcome-container").hide();
    $.pjax.submit(ev, pjaxOpt || {container: $("[data-pjax-container]")});
  } else {
    if (ev) ev.preventDefault();
    $("#fake-results-container").hide().removeClass("vis");
    $("#results-container").hide();
    $("#welcome-container").show().addClass("vis");
    var newURL = window.location.protocol + "//" + window.location.host + window.location.pathname;
    window.history.pushState(null, "", newURL);
  }
}

// PJAX
$(document).on("submit", "form[data-pjax]", function(event) {
  search(ev, document.getElementById("q").value);
});
document.addEventListener("pjax:end", function() {
  highlightQuery();
  $("[autofocus]").focus();
});


$.pjax.defaults.timeout = 60000; // Time out after 60 seconds.
$(document).on("pjax:send", function(ev) {
  $("#welcome-container").hide();
  $("#fake-results-container").show().addClass("vis");
  $("#results-container").hide();
});
$(document).on("pjax:complete", function(ev) {
  $("#fake-results-container").hide().removeClass("vis");
  $("#results-container").show();
});

// Hack to allow document.addEventListener("pjax:end", ...) listeners
// to be triggered. Without this, you need to use jQuery to listen to
// the event.
$(document).on("pjax:end", function(ev) {
  if (!ev.originalEvent) document.dispatchEvent(new Event("pjax:end"));
});


// Use PJAX for popular queries.
document.addEventListener("DOMContentLoaded", function() {
  var popularQueriesElems = document.querySelectorAll(".popular-queries");
  for (var i = 0; i < popularQueriesElems.length; i++) {
    $(popularQueriesElems[i]).pjax("a", "[data-pjax-container]");

    // Update the query field with the popular query after a click.
    popularQueriesElems[i].addEventListener("click", function(ev) {
      var $a = ev.target;
      var $q = document.getElementById("q");
      $q.value = $a.dataset.query;
      $q.focus();
    });
  }
});

// Automatically start searching after typing.
document.addEventListener("DOMContentLoaded", function() {
  var $q = document.getElementById("q");

  var debouncedSearch = debounce(function() {
    var fakeEvent = {currentTarget: $q.form, type: "submit", preventDefault: function() {}};
    search(fakeEvent, $q.value.trim(), {blurInput: false, container: $("[data-pjax-container]"), replace: true});
  }, 200, false);
  var handleKeyInput = function(ev) {
    if (!event.charCode) return;
    if (String.fromCharCode(ev.charCode) == " ") return;
    setTimeout(function() {
      var q = $q.value.trim();
      if (q) {
        $("#welcome-container").hide();
        $("#results-container").hide();
        $("#fake-results-container").show().addClass("vis");
        debouncedSearch();
      } else {
        search(null, null); // reset everything
      }
    });
  }
  
  $($q).on("keypress", handleKeyInput);
  $($q).on("keyup", function(event) {
    // DELETE or BACKSPACE key.
    if (event.which == 8 || event.which == 46 || true) handleKeyInput(event);
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


// Animate welcome on page load, if there's no search.
document.addEventListener("DOMContentLoaded", function() {
  setTimeout(function() {
    if (!document.getElementById("q").value.trim()) {
      document.getElementById("welcome-container").classList.add("vis");
    }
  });
});

// {{end}}
