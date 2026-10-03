Output .github/assets/demo.gif
Set FontSize 14
Set Width 1400
Set Height 580
Set Theme "Catppuccin Mocha"
Set TypingSpeed 120ms

Hide
Type "KAPI_BIN=$PWD/kapi && mkdir -p ~/kapi-demo && cd ~/kapi-demo && export HOME=$(mktemp -d) && git config --global user.name Demo && git config --global user.email demo@example.com && alias kapi=$KAPI_BIN && clear"
Enter
Show

Sleep 1s
Type "kapi"
Sleep 800ms
Enter

# Welcome
Sleep 3.5s
Enter

# Folder: custom path
Sleep 1s
Down
Sleep 600ms
Enter
Sleep 600ms
Type "myapp"
Sleep 1s
Enter

# Ecosystem: PHP
Sleep 1.5s
Enter

# Framework: browse the list, keep Laravel
Sleep 2.5s
Down
Sleep 900ms
Down
Sleep 900ms
Up
Sleep 700ms
Up
Sleep 1s
Enter

# Packages
Sleep 4.5s
Space
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 700ms
Space
Sleep 2s
Enter

# Git: local repo, collab, CI
Sleep 1.5s
Right
Sleep 1.2s
Down
Down
Down
Down
Sleep 700ms
Right
Sleep 900ms
Down
Sleep 700ms
Right
Sleep 1.5s
Enter

# Summary
Sleep 4s
