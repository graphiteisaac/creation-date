FROM python:3.9-slim

WORKDIR /usr/src/app

COPY . .
# RUN pip install --no-cache-dir

CMD ["python", "User_Date_bot.py"]
