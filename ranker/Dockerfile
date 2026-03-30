FROM python:3.12-slim

# LightGBM requires libgomp for OpenMP parallelism at runtime.
# python:3.12-slim doesn't include it, so install before pip.
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Railway injects PORT; fall back to 8000 for local runs.
ENV PORT=8000
EXPOSE $PORT

# Bind to 0.0.0.0 so Railway can route traffic to the container.
CMD ["sh", "-c", "uvicorn main:app --host 0.0.0.0 --port $PORT --log-level info"]
