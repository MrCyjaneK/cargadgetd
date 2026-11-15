# Sailfish UI for the cargadgetd Android Auto phone daemon.
# The RPM spec builds this with qmake and installs the Go binaries itself.
TARGET = cargadgetd

CONFIG += sailfishapp c++11
SAILFISHAPP_ICONS = 86x86 108x108 128x128 172x172 256x256

QT += core qml quick concurrent
LIBS += -ldl

SOURCES += \
    src/main.cpp \
    src/daemoncontroller.cpp \
    src/pluginsettings.cpp

HEADERS += \
    src/daemoncontroller.h \
    src/pluginsettings.h

OTHER_FILES += \
    qml/*.qml \
    qml/pages/*.qml \
    qml/cover/*.qml \
    cargadgetd.desktop \
    usb-aoa-launch.sh \
    systemd/cargadgetd-aoa.service \
    icons/cargadgetd.svg \
    icons/86x86/cargadgetd.png \
    icons/108x108/cargadgetd.png \
    icons/128x128/cargadgetd.png \
    icons/172x172/cargadgetd.png \
    icons/256x256/cargadgetd.png

# AOA waiter. The usb-moded ask dialog stays up until the host sends START.
usb_mode_unit.files = systemd/cargadgetd-aoa.service
usb_mode_unit.path = /usr/lib/systemd/system
INSTALLS += usb_mode_unit

usb_mode_launch.files = usb-aoa-launch.sh
usb_mode_launch.path = /usr/share/$${TARGET}
INSTALLS += usb_mode_launch
