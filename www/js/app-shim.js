// Remplace l'ancien blob Haxe www/js/app.js (18820 lignes générées par Haxe
// 4.0.5). Seuls 3 points d'entrée étaient appelés depuis les templates Go :
//   - _.antiDoubleClick()
//   - _.shop(id)        → désormais une redirection serveur
//   - _.loginBox(url)   → code mort dans ce port (jamais déclenché par Go)
// Ce shim expose le namespace _ pour que les templates qui font
// `_.lang = "fr"` ou `_.userId = N` ne plantent pas.
(function () {
  "use strict";

  var ns = (window._ = window._ || {});

  // Désactive un bouton ~3s après clic pour empêcher les doubles soumissions.
  // Port direct de App.hx::antiDoubleClick (Haxe).
  ns.antiDoubleClick = function () {
    var btns = document.querySelectorAll(".btn:not(.btn-noAntiDoubleClick)");
    for (var i = 0; i < btns.length; i++) {
      btns[i].addEventListener("click", function (e) {
        var t = e.currentTarget;
        t.classList.add("disabled");
        setTimeout(function () {
          t.classList.remove("disabled");
        }, 3000);
      });
    }
  };

  // No-ops défensifs : si l'un de ces points d'entrée est rappelé par une
  // vieille page templatée, on échoue silencieusement plutôt que de jeter une
  // ReferenceError. Le shop est servi par la SPA React sous /shop2/ et le
  // serveur ne devrait pas appeler loginBox.
  ns.shop = function () {
    if (window.console) console.warn("[app-shim] _.shop() est obsolète — la boutique est désormais /shop2/.");
  };
  ns.loginBox = function () {
    if (window.console) console.warn("[app-shim] _.loginBox() est obsolète.");
  };
})();
