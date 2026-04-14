# This is a VHS script to generate a beautiful GIF of KAPI in action.
# To run it, you need to install VHS: https://github.com/charmbracelet/vhs
# Then simply type: make demo

Output .github/assets/demo.gif

Require kapi

Set Margin 20
Set MarginFill "#674EFF"
Set BorderRadius 10
Set WindowBar Rings

Set FontSize 18
Set Width 1200
Set Height 800
Set TypingSpeed 0.1

Type "kapi"
Sleep 500ms
Enter

Sleep 2.5s

# Animate browsing to "Start a new project"
Enter
Sleep 1s

# Type a project name
Type "my-awesome-app"
Sleep 500ms
Enter
Sleep 1s

# Navigate down to select a framework (e.g. React or Next)
Down 2
Sleep 500ms
Enter
Sleep 1s

# Final suspense
Sleep 4s
