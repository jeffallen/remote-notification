# Guide de Localisation / Localization Guide

## Français

### Fonctionnalité ajoutée

L'application Android Remote Notify Demo App supporte maintenant le français ! L'application détecte automatiquement la langue du système et affiche l'interface dans la langue appropriée.

### Langues supportées

- **Anglais** (par défaut) - `values/strings.xml`
- **Français** - `values-fr/strings.xml`

### Comment tester

1. **Pour tester en français :**
   - Allez dans les Paramètres de votre appareil Android
   - Sélectionnez "Langues et entrée" ou "Language & input"
   - Changez la langue principale vers "Français"
   - Redémarrez l'application

2. **Pour tester en anglais :**
   - Remettez la langue de l'appareil en "English"
   - Redémarrez l'application

### Éléments traduits

- Titre de l'application
- Interface principale (enregistrement du token)
- Écran des paramètres
- Messages d'état et d'erreur
- Messages toast (notifications temporaires)
- Menu et navigation

### Ajouter d'autres langues

Pour ajouter une nouvelle langue (ex: espagnol) :

1. Créez un nouveau dossier : `app/src/main/res/values-es/`
2. Copiez le fichier `strings.xml` depuis `values/`
3. Traduisez toutes les chaînes de caractères
4. Testez avec un appareil configuré en espagnol

---

## English

### Added Feature

The Remote Notify Demo App Android application now supports French! The app automatically detects the system language and displays the interface in the appropriate language.

### Supported Languages

- **English** (default) - `values/strings.xml`
- **French** - `values-fr/strings.xml`

### How to Test

1. **To test in French:**
   - Go to your Android device Settings
   - Select "Languages and input"
   - Change the primary language to "Français" (French)
   - Restart the application

2. **To test in English:**
   - Change device language back to "English"
   - Restart the application

### Translated Elements

- Application title
- Main interface (token registration)
- Settings screen
- Status and error messages
- Toast messages (temporary notifications)
- Menu and navigation

### Adding Other Languages

To add a new language (e.g., Spanish):

1. Create a new folder: `app/src/main/res/values-es/`
2. Copy the `strings.xml` file from `values/`
3. Translate all string values
4. Test with a device configured in Spanish

---

## Technical Implementation

### Structure

```
app/src/main/res/
├── values/
│   └── strings.xml          # English (default)
├── values-fr/
│   └── strings.xml          # French
└── layout/
    ├── activity_main.xml    # Uses @string/ references
    └── activity_settings.xml # Uses @string/ references
```

### Code Changes

- All hardcoded strings moved to resource files
- Kotlin code updated to use `getString(R.string.resource_name)`
- Layout files updated to use `@string/resource_name`
- Proper parameter formatting with `%1$s`, `%1$d` for dynamic content

### Best Practices

- Always use string resources instead of hardcoded text
- Use proper parameter formatting for dynamic strings
- Test all languages on actual devices
- Keep string keys consistent and descriptive
- Add comments to organize string resources
