FROM mcr.microsoft.com/dotnet/sdk:9.0-alpine AS build
WORKDIR /src
COPY . .
RUN dotnet publish src/DomainService.FunctionApp/DomainService.FunctionApp.csproj -c Release -o /tmp/domain-service

FROM mcr.microsoft.com/azure-functions/dotnet-isolated:4-dotnet-isolated9.0
WORKDIR /home/site/wwwroot
COPY --from=build /tmp/domain-service .