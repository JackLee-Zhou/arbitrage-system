# services/llm_scorer/Dockerfile
FROM python:3.12-slim

WORKDIR /app

# 安装 uv
RUN pip install --no-cache-dir uv

# 安装依赖
COPY pyproject.toml ./
RUN uv pip install --system -r <(uv pip compile pyproject.toml -o -)

COPY . .

EXPOSE 5000
CMD ["python", "main.py"]
