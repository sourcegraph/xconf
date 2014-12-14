// {{define "JS"}}

// Query highlighting
function highlightQuery() {
  var resultsList = document.getElementById("results");
  if (!resultsList || resultsList.childNodes.length == 0) return; // nothing to highlight
  var q = document.getElementById("q").value;
  if (q) {
    q.split(/[^\w]/).forEach(function(p) {
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
  $("#fake-results-container").show();
  $("#results-container").hide();
});
$(document).on("pjax:complete", function(ev) {
  $("#fake-results-container").hide();
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

// {{end}}
