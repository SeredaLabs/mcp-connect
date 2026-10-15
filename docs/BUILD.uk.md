# 🛠️ Збирання MCP Connect

🌐 [English](BUILD.md) · **Українська**

[← Огляд](INSTALL.uk.md) · [Авторизація](AUTHENTICATION.uk.md) · [Безпека](SECURITY.uk.md) · [Збирання](BUILD.uk.md)

Збирайте пакунок для відповідної ОС із версіями залежностей, зафіксованими в `go.mod` і `go.sum`.

## 📋 Вимоги

| Платформа | Інструменти |
|---|---|
| macOS | Go 1.27.2+, Xcode Command Line Tools |
| Windows AMD64 | Go 1.27.2+, Git Bash, MSYS2 UCRT64 GCC |
| Windows ARM64 | Go 1.27.2+, Git Bash, MSYS2 CLANGARM64 clang |
| Ubuntu 24.04 | Go 1.27.2+, компілятор C, бібліотеки для графічного інтерфейсу |

Для Windows потрібен `7z` або `zip` у PATH. Установіть `CC` у повний шлях до відповідного компілятора MSYS2, щоб випадково не використати інший компілятор із PATH.

Для Ubuntu встановіть:

```sh
sudo apt-get install build-essential libgl1-mesa-dev libegl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols
```

## 🛠️ Збирання

```sh
bash scripts/build.sh
```

Пакунки зберігаються в `dist/`. До них входять виконуваний файл або `.app`, українська інструкція, ліцензія програми та повідомлення про ліцензії залежностей.

[Процес збирання](https://github.com/SeredaLabs/mcp-connect/actions/workflows/build.yml) створює шість пакунків на відповідних ОС і перевіряє цілісність залежностей, статичний аналіз та відомі вразливості. Він не виконує підписування видавцем чи нотаризацію.

Пакування Windows створює вбудовану іконку через `go-winres v0.3.3`. Цей інструмент завантажується Go лише для збирання й не входить до програми.
