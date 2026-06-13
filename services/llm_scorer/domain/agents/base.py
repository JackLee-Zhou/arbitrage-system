"""Agent 基类 — 所有评分 Agent 的抽象基类"""
from abc import ABC, abstractmethod


class BaseAgent(ABC):
    """评分 Agent 基类"""

    def __init__(self, name: str):
        self.name = name

    @abstractmethod
    async def analyze(self, context: dict) -> dict:
        """分析输入上下文，返回评分结果

        Returns:
            dict: {"score": float, "confidence": float, "reasoning": str, ...}
        """
        ...

    @abstractmethod
    def prompt_template(self) -> str:
        """返回该 Agent 的 Prompt 模板"""
        ...
