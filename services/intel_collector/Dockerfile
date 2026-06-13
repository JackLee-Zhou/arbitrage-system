# services/intel_collector/Dockerfile
FROM python:3.12-slim

WORKDIR /app

RUN pip install --no-cache-dir uv

COPY pyproject.toml ./
RUN uv pip install --system -r <(uv pip compile pyproject.toml -o -)

COPY . .

CMD ["python", "main.py"]
