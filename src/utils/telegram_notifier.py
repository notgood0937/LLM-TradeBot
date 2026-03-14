import threading
from typing import Optional

import requests

from src.utils.logger import log


class TelegramNotifier:
    """Simple Telegram notifier for decision messages."""

    def __init__(
        self,
        *,
        enabled: bool,
        bot_token: Optional[str],
        chat_id: Optional[str],
        timeout: int = 10
    ):
        self.enabled = bool(enabled)
        self.bot_token = bot_token
        self.chat_id = str(chat_id) if chat_id is not None else None
        self.timeout = timeout

    @classmethod
    def from_config(cls, config) -> "TelegramNotifier":
        cfg = getattr(config, "telegram", {}) or {}
        return cls(
            enabled=bool(cfg.get("enabled", False)),
            bot_token=cfg.get("bot_token"),
            chat_id=cfg.get("chat_id"),
            timeout=int(cfg.get("timeout", 10) or 10)
        )

    def is_ready(self) -> bool:
        return self.enabled and bool(self.bot_token) and bool(self.chat_id)

    def send_message(self, text: str) -> bool:
        if not self.is_ready():
            return False
        url = f"https://api.telegram.org/bot{self.bot_token}/sendMessage"
        payload = {
            "chat_id": self.chat_id,
            "text": text,
            "disable_web_page_preview": True
        }
        try:
            resp = requests.post(url, json=payload, timeout=self.timeout)
            resp.raise_for_status()
            return True
        except Exception as exc:
            log.warning(f"Telegram notify failed: {exc}")
            return False

    def send_message_async(self, text: str) -> bool:
        if not self.is_ready():
            return False
        thread = threading.Thread(target=self.send_message, args=(text,), daemon=True)
        thread.start()
        return True
