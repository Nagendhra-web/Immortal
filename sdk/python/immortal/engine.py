from dataclasses import dataclass, field
from typing import Callable
import sys
import threading

from .event import Event, Severity


@dataclass
class HealingRule:
    name: str
    match: Callable[[Event], bool]
    action: Callable[[Event], None]


@dataclass
class Config:
    name: str = "immortal-app"
    mode: str = "reactive"
    ghost_mode: bool = False


class Immortal:
    def __init__(self, name: str = "immortal-app", mode: str = "reactive",
                 ghost_mode: bool = False):
        self.config = Config(name=name, mode=mode, ghost_mode=ghost_mode)
        self._rules: list[HealingRule] = []
        self._events: list[Event] = []
        self._running = False
        self._lock = threading.Lock()

    def add_rule(self, rule: HealingRule) -> None:
        self._rules.append(rule)

    def heal(self, name: str, when: Callable[[Event], bool],
             do: Callable[[Event], None]) -> None:
        self.add_rule(HealingRule(name=name, match=when, action=do))

    def healer(self, name: str):
        """Decorator for healing rules."""
        def decorator(func):
            self.add_rule(HealingRule(
                name=name,
                match=lambda e: e.severity.level >= Severity.ERROR.level,
                action=func,
            ))
            return func
        return decorator

    def handle_event(self, event: Event) -> list[str]:
        with self._lock:
            self._events.append(event)

        matched = []
        for rule in self._rules:
            if rule.match(event):
                matched.append(rule.name)
                if not self.config.ghost_mode:
                    rule.action(event)

        return matched

    def start(self) -> None:
        self._running = True

    def stop(self) -> None:
        self._running = False

    @property
    def is_running(self) -> bool:
        return self._running

    @property
    def events(self) -> list[Event]:
        with self._lock:
            return list(self._events)
