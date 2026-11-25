# IBM Plex Serif

`IBMPlexSerif.woff2` is IBM Plex Serif, version 2.6, the serif font of
zuzu (D35 part 11, D58 part 1). It is under the SIL Open Font License 1.1,
which `LICENSE.txt` holds. The license reserves the font name Plex.

The file is one collection of six faces in WOFF2, in this order: Regular
(400), Medium (500), SemiBold (600), Italic (400), MediumItalic (500) and
SemiBoldItalic (600). The six faces are the static TrueType files of
<https://github.com/google/fonts>, in the folder `ofl/ibmplexserif` of the
commit `ddd42eafb287178377d9db887a3d0b8d368d824b`, unchanged:

| File | SHA-256 |
| --- | --- |
| `IBMPlexSerif-Regular.ttf` | `e882efa9c41949a528ac2369079ec5ef050c1c996bbd0bacce3c3326d44cf80d` |
| `IBMPlexSerif-Medium.ttf` | `32925e73e5b4a441df9d23e847f2b8c4046c1fc3db094e90bad8883157811509` |
| `IBMPlexSerif-SemiBold.ttf` | `27ce4d105591b48f666b228e1e399838a704594efea22b730000951bea700a60` |
| `IBMPlexSerif-Italic.ttf` | `4b75b38be4673527231f49c48818d090c913d5042dd5c747b525bf6185d29ecb` |
| `IBMPlexSerif-MediumItalic.ttf` | `a3b5c5b291db0b88abf833bd02864adcaa4ba994d9f7d9b790f552e2fd0cbabd` |
| `IBMPlexSerif-SemiBoldItalic.ttf` | `15f4e3974b83b2459a6a1b252caa356a9a11316deeba45adbc89b46fb4aab820` |

`TTCollection` of fontTools 4.68.0 joins the six files into one TrueType
collection, and shares the tables that are the same in each face. The
collection holds 985,012 bytes, and its SHA-256 is
`24620f0034d67fe6b597f4936b4f2e74b3a0f533446e3bee2500d84deacabfcc`.
`woff2_compress` of the Google WOFF2 tools then compresses it, as D34 did
for IBM Plex Sans. `IBMPlexSerif.woff2` holds 220,172 bytes, and its
SHA-256 is
`94d6af8ee623a2bd1234173a2c008bba0a563e73981951bc294f6da49779a54f`.

`LICENSE.txt` is the file `OFL.txt` of the same folder of google/fonts.
