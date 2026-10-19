FROM php:8.2-apache

# mysqli is what the app uses for all the DB calls.
# curl is already built into this image so it doesn't need to be installed.
RUN docker-php-ext-install mysqli

# The app links to /WeatherApp/... everywhere, so it gets mounted there
RUN mkdir -p /var/www/html/WeatherApp

COPY apache.conf /etc/apache2/conf-available/weatherapp.conf
RUN a2enconf weatherapp

EXPOSE 80
