FROM mcr.microsoft.com/azure-functions/python:4-python312

ENV AzureWebJobsScriptRoot=/home/site/wwwroot \
    AzureFunctionsJobHost__Logging__Console__IsEnabled=true \
    FUNCTIONS_WORKER_RUNTIME=python

WORKDIR /home/site/wwwroot

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
