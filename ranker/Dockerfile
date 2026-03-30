FROM python:3.12-slim

# Install deps before copying source so layer is cached across code changes.
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Railway injects PORT; fall back to 8000 for local runs.
ENV PORT=8000
EXPOSE $PORT

# Bind to 0.0.0.0 so Railway can route traffic to the container.
CMD ["sh", "-c", "uvicorn main:app --host 0.0.0.0 --port $PORT --log-level info"]
