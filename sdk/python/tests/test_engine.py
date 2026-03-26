import pytest
from immortal import Immortal, Event, Severity, EventType


def test_default_config():
    app = Immortal()
    assert app.config.name == "immortal-app"
    assert app.config.mode == "reactive"
    assert app.config.ghost_mode is False


def test_custom_config():
    app = Immortal(name="my-api", mode="autonomous")
    assert app.config.name == "my-api"
    assert app.config.mode == "autonomous"


def test_start_stop():
    app = Immortal()
    app.start()
    assert app.is_running is True
    app.stop()
    assert app.is_running is False


def test_healing_rule_matches():
    app = Immortal()
    healed = []

    app.heal(
        name="restart-on-crash",
        when=lambda e: e.severity.level >= Severity.CRITICAL.level,
        do=lambda e: healed.append(e.message),
    )

    event = Event(
        type=EventType.ERROR,
        severity=Severity.CRITICAL,
        message="process crashed",
        source="test",
    )

    matched = app.handle_event(event)
    assert "restart-on-crash" in matched
    assert healed == ["process crashed"]


def test_ghost_mode_no_execution():
    app = Immortal(ghost_mode=True)
    executed = []

    app.heal(
        name="test-rule",
        when=lambda e: e.severity.level >= Severity.CRITICAL.level,
        do=lambda e: executed.append(True),
    )

    event = Event(
        type=EventType.ERROR,
        severity=Severity.CRITICAL,
        message="crash",
    )

    matched = app.handle_event(event)
    assert "test-rule" in matched
    assert executed == []


def test_events_stored():
    app = Immortal()
    event = Event(
        type=EventType.ERROR,
        severity=Severity.ERROR,
        message="something broke",
    )

    app.handle_event(event)
    assert len(app.events) == 1
    assert app.events[0].message == "something broke"


def test_severity_ordering():
    assert Severity.CRITICAL.level > Severity.WARNING.level
    assert Severity.WARNING.level > Severity.INFO.level
    assert Severity.FATAL.level > Severity.CRITICAL.level
