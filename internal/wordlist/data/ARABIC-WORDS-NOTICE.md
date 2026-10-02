# arabic-words.txt.gz

Arabic word list used for Arabic prompts, one word per line, most common first.

## How it was made

1. Start from the Arabic word frequency list `ar_full.txt` of
   [FrequencyWords](https://github.com/hermitdave/FrequencyWords) by Hermit Dave
   (built from OpenSubtitles 2018).
2. Strip diacritics (tashkeel) and tatweel, and keep words made only of Arabic
   letters, at least 2 letters long, seen at least 3 times.
3. Keep only the words accepted by the
   [Ayaspell](http://ayaspell.sourceforge.net/) Hunspell dictionary (`ar.aff`,
   `ar.dic` from [LibreOffice/dictionaries](https://github.com/LibreOffice/dictionaries/tree/master/ar)),
   checked with [spylls](https://github.com/zverok/spylls).

## Licences

- The word list (an adaptation of FrequencyWords content) is licensed under
  [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Credit:
  Hermit Dave, FrequencyWords. This licence covers this file only, not the rest
  of the project (MIT).
- The Ayaspell dictionary used for filtering is available under GPL 2.0 /
  LGPL 2.1 / MPL 1.1; none of its files are included here.
