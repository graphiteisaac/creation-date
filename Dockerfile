FROM python:3.9-slim

WORKDIR /usr/src/app

COPY . .

# install deps
RUN python3 -m pip install -U pyyaml
RUN python3 -m pip install -U discord.py
RUN python3 -m pip install -U python-dotenv

CMD ["python", "User_Date_Bot.py"]
