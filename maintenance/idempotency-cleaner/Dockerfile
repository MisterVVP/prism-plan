FROM mcr.microsoft.com/azure-functions/python:4-python3.12

WORKDIR /home/site/wwwroot

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
