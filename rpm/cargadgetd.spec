%global _missing_build_ids_terminate_build 0

Name:           cargadgetd
Version:        0.1
Release:        7
Summary:        Android Auto phone stack and Sailfish control UI
Group:          Applications/System
License:        GPLv3
Source0:        %{name}-%{version}.tar.bz2
BuildRequires:  gcc
BuildRequires:  gcc-c++
BuildRequires:  golang
BuildRequires:  make
BuildRequires:  openssl-devel
BuildRequires:  pkgconfig(sailfishapp)
BuildRequires:  pkgconfig(Qt5Quick)
BuildRequires:  pkgconfig(Qt5Qml)
BuildRequires:  pkgconfig(Qt5Core)
BuildRequires:  pkgconfig(Qt5Concurrent)
BuildRequires:  pkgconfig(qdeclarative5-boostable)
BuildRequires:  desktop-file-utils
Requires:       mapplauncherd-booster-silica-qt5
Requires:       sailfishsilica-qt5
Obsoletes:      reversegearhead

%description
Cargadgetd control UI


%prep
%setup -q


%build
set -eu
src=$(pwd)
cd ui/sfos/cargadgetd
%qmake5
make %{?_smp_mflags}
cd "$src"

export PATH="/usr/local/go/bin:${PATH}"
export GOTOOLCHAIN=local
export GO111MODULE=on
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"
export CGO_ENABLED=1

mkdir -p _out/bin
go build -buildvcs=false -tags "netgo osusergo" \
    -ldflags "-s -w -linkmode external -extldflags '-ldl'" \
    -o _out/bin/cargadgetd-md ./cmd/cargadgetd
CGO_ENABLED=0 go build -buildvcs=false -ldflags "-s -w" \
    -o _out/bin/cargadgetd-gadget ./cmd/gadgetctl
go build -buildvcs=false -buildmode=c-shared -ldflags "-s -w" \
    -o _out/libcargadgetd_app_daemon.so ./cmd/daemonsettings
go build -buildvcs=false -buildmode=c-shared -ldflags "-s -w" \
    -o _out/libcargadgetdcontrol.so ./cmd/control

%install
set -eu
rm -rf %{buildroot}
src=$(pwd)
cd ui/sfos/cargadgetd
%qmake5_install
cd "$src"

chmod 755 %{buildroot}%{_datadir}/%{name}/usb-aoa-launch.sh
install -m 755 scripts/music-loopback-restore.sh %{buildroot}%{_datadir}/%{name}/music-loopback-restore.sh

install -d %{buildroot}%{_datadir}/%{name}/bin
install -m 755 _out/bin/cargadgetd-md %{buildroot}%{_datadir}/%{name}/bin/cargadgetd-md
install -m 755 _out/bin/cargadgetd-gadget %{buildroot}%{_datadir}/%{name}/bin/cargadgetd-gadget

install -d %{buildroot}/usr/lib/cargadgetd/plugins
install -m 755 _out/libcargadgetd_app_daemon.so %{buildroot}/usr/lib/cargadgetd/plugins/libcargadgetd_app_daemon.so
install -m 755 _out/libcargadgetdcontrol.so %{buildroot}/usr/lib/cargadgetd/libcargadgetdcontrol.so

desktop-file-install --delete-original \
    --dir %{buildroot}%{_datadir}/applications \
    %{buildroot}%{_datadir}/applications/*.desktop


%post
systemctl daemon-reload >/dev/null 2>&1 || :
systemctl disable --now cargadgetd-usb-mode.service >/dev/null 2>&1 || :
systemctl disable --now cargadgetd-usb-whitelist.service >/dev/null 2>&1 || :
dbus-send --system --type=method_call --dest=com.meego.usb_moded \
    /com/meego/usb_moded com.meego.usb_moded.set_whitelisted \
    string:cargadgetd boolean:false >/dev/null 2>&1 || :
if [ -f /var/lib/usb-moded/usb-moded.ini ]; then
    wl=$(sed -n 's/^whitelist=//p' /var/lib/usb-moded/usb-moded.ini | head -n1 || true)
    new=$(printf '%s\n' "$wl" | tr ',' '\n' | grep -v -x cargadgetd | paste -sd, - || true)
    sed -i "s/^whitelist=.*/whitelist=${new}/" /var/lib/usb-moded/usb-moded.ini || :
fi
systemctl enable cargadgetd-aoa.service >/dev/null 2>&1 || :
systemctl restart cargadgetd-aoa.service >/dev/null 2>&1 || :
systemctl try-restart usb-moded.service >/dev/null 2>&1 || :

%preun
if [ "$1" -eq 0 ]; then
systemctl disable --now cargadgetd-usb-mode.service >/dev/null 2>&1 || :
systemctl disable --now cargadgetd-usb-whitelist.service >/dev/null 2>&1 || :
systemctl disable --now cargadgetd-aoa.service >/dev/null 2>&1 || :
fi

%postun
if [ "$1" -eq 0 ]; then
systemctl daemon-reload >/dev/null 2>&1 || :
systemctl try-restart usb-moded.service >/dev/null 2>&1 || :
fi


%files
%defattr(-,root,root,-)
%{_bindir}/%{name}
%{_datadir}/applications/%{name}.desktop
%{_datadir}/%{name}/qml
%dir %{_datadir}/%{name}
%dir %{_datadir}/%{name}/bin
%{_datadir}/%{name}/bin/cargadgetd-md
%attr(4755,root,root) %{_datadir}/%{name}/bin/cargadgetd-gadget
%{_datadir}/icons/hicolor/*/apps/%{name}.png
/usr/lib/systemd/system/cargadgetd-aoa.service
%attr(755,root,root) %{_datadir}/%{name}/usb-aoa-launch.sh
%attr(755,root,root) %{_datadir}/%{name}/music-loopback-restore.sh
%dir /usr/lib/cargadgetd
%dir /usr/lib/cargadgetd/plugins
/usr/lib/cargadgetd/libcargadgetdcontrol.so
/usr/lib/cargadgetd/plugins/libcargadgetd_app_daemon.so
