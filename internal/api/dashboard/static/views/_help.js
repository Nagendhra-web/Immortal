// _help.js — keyboard shortcuts overlay.

import { h } from "../lib/fmt.js";
import { dialog } from "../components/dialog.js";
import { HELP_SHORTCUTS } from "../lib/keyboard.js";

export function openHelp() {
  const list = h("dl", { class: "help-grid" });
  for (const [k, desc] of HELP_SHORTCUTS) {
    list.appendChild(h("dt", {}, k));
    list.appendChild(h("dd", {}, desc));
  }
  dialog({
    title: "Keyboard shortcuts",
    body: h("div", {},
      h("p", { class: "muted", style: { marginTop: 0 } }, "Most shortcuts work anywhere except inside text inputs."),
      list,
    ),
  });
}
