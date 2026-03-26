from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any
import secrets


class EventType(str, Enum):
    ERROR = "error"
    METRIC = "metric"
    LOG = "log"
    TRACE = "trace"
    HEALTH = "health"


class Severity(str, Enum):
    DEBUG = "debug"
    INFO = "info"
    WARNING = "warning"
    ERROR = "error"
    CRITICAL = "critical"
    FATAL = "fatal"

    @property
    def level(self) -> int:
        return list(Severity).index(self)


@dataclass
class Event:
    type: EventType
    severity: Severity
    message: str
    id: str = field(default_factory=lambda: secrets.token_hex(16))
    source: str = ""
    timestamp: datetime = field(default_factory=datetime.utcnow)
    meta: dict[str, Any] = field(default_factory=dict)
