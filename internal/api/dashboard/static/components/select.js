// select.js — popover-based enhanced select.
// select({ label, value, options, onChange })

import { h } from "../lib/fmt.js";
import { popover } from "./popover.js";

export function select({ label, value, options, onChange, placeholder = "Select…" } = {}) {
  const current = options.find((o) => o.value === value) || options[0];
  const valueSpan = h("span", { class: "select__value" }, current?.label ?? placeholder);
  const trigger = h("button", {
    class: "select",
    type: "button",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
    onClick: () => open(),
  },
    label ? h("span", { class: "select__label" }, label) : null,
    valueSpan,
    h("span", { class: "select__chevron" }, "▾"),
  );

  function open() {
    trigger.setAttribute("aria-expanded", "true");
    const list = h("ul", { class: "popover__list", role: "listbox" },
      ...options.map((o) =>
        h("li", {
          role: "option",
          class: "popover__item",
          "aria-selected": o.value === value ? "true" : "false",
          onClick: () => { select(o); },
        },
          o.icon ? h("span", {}, o.icon) : null,
          o.label,
        )
      ),
    );
    const pop = popover({
      anchor: trigger,
      content: list,
      onClose: () => trigger.setAttribute("aria-expanded", "false"),
    });
    function select(o) {
      value = o.value;
      valueSpan.textContent = o.label;
      pop.close();
      onChange && onChange(o.value, o);
    }
  }

  return { el: trigger, setValue(v) {
    const o = options.find((x) => x.value === v);
    if (!o) return;
    value = v;
    valueSpan.textContent = o.label;
  } };
}
