CREATE DATABASE questiongo;
CREATE TABLE questiongo.data (id BIGINT UNSIGNED AUTO_INCREMENT, questionnaire VARCHAR(200) NOT NULL, question VARCHAR(200) NOT NULL, data LONGTEXT NOT NULL, PRIMARY KEY(id));
CREATE INDEX qda ON questiongo.data (questionnaire,question);
