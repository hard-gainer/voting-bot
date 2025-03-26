
Необходимо добавить функционал для системы голосования внутри чатов мессенджера Mattermost.
Бот должен позволять пользователям создавать голосования, голосовать за предложенные варианты 
и просматривать результаты. 

### Функциональные требования: 
1. Создание голосования (Бот регистрирует голосование и возвращает сообщение с ID голосования и вариантами ответов). 
2. Голосование (Пользователь отправляет команду, указывая ID голосования и вариант ответа). 
3. Просмотр результатов (Любой пользователь может запросить текущие результаты голосования). 
4. Завершение голосования (Создатель голосования может завершить его досрочно). 
5. Удаление голосования (Возможность удаления голосования). 

### Нефункциональные требования: 
1. Код должен быть написан на Go.  
2. Логирование действий. 
3. Хранение данных в Tarantool. 
4. Использование docker и docker-compose для поднятия и развертывания dev-среды. 
5. Код должен быть выложен на github или аналог. Код должен быть сопровожден инструкцией по сборке и установке.