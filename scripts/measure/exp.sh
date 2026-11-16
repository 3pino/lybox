#!/bin/bash
# usage: exp.sh <label> <mode> <apps: all|nodisnans|none> <seconds>
L=$1; MODE=$2; APPS=$3; S=$4
UID_TS=${UID_TS:-10366}
PK_D=dev.disnans.app; PK_S=com.github.catfriend1.syncthingandroid; PK_K=org.kde.kdeconnect_tp
cd /tmp/bt; mkdir -p exp
adb shell "run-as dev.lybox.ts sh -c 'rm -f files/debug_traffic; echo $MODE > files/power_mode'"
if [ -n "$WIFI" ]; then adb shell svc wifi enable; else adb shell svc wifi disable; fi
for p in $PK_D $PK_S $PK_K; do adb shell am force-stop $p; done
case $APPS in
 all) for p in $PK_D $PK_S $PK_K; do adb shell monkey -p $p -c android.intent.category.LAUNCHER 1 >/dev/null 2>&1; sleep 3; done;;
 nodisnans) for p in $PK_S $PK_K; do adb shell monkey -p $p -c android.intent.category.LAUNCHER 1 >/dev/null 2>&1; sleep 3; done;;
esac
adb shell input keyevent KEYCODE_HOME
sleep 5; adb shell input keyevent KEYCODE_SLEEP
[ -n "$WIFI" ] && sleep 20
sleep 40   # let the policy apply, DERP reconnect and app reconnects settle
adb shell dumpsys battery unplug
adb shell dumpsys batterystats --reset >/dev/null
adb shell "rm -f /data/local/tmp/net.log; nohup sh -c 'i=0; while [ \$i -lt $S ]; do echo \$(date +%s) \$(grep rmnet1: /proc/net/dev); sleep 1; i=\$((i+1)); done' > /data/local/tmp/net.log 2>&1 &"
sleep $((S+4))
adb shell dumpsys batterystats --checkin | grep -E "^9,$UID_TS,l,(pwi|nt)" > exp/$L.bs
adb pull /data/local/tmp/net.log exp/$L.net >/dev/null 2>&1
adb shell dumpsys battery reset
adb logcat -d | grep -a "power: mode" | tail -1 | sed -E 's/.*power: //' | cut -c1-80 > exp/$L.pol
python3 - "$L" <<'PY'
import sys,re
L=sys.argv[1]
rows=[]
for ln in open(f'/tmp/bt/exp/{L}.net'):
    f=ln.split()
    if len(f)<12 or 'rmnet1' not in f[1]: continue
    t=int(f[0]); rx_b=int(f[2]); rx_p=int(f[3]); tx_b=int(f[10]); tx_p=int(f[11])
    rows.append((t,rx_b+tx_b,rx_p+tx_p))
if len(rows)<2: print(L,'no data'); sys.exit()
pk=rows[-1][2]-rows[0][2]; by=rows[-1][1]-rows[0][1]; dur=rows[-1][0]-rows[0][0]
# seconds in which counters moved
active=[rows[i][0] for i in range(1,len(rows)) if rows[i][2]!=rows[i-1][2]]
TAIL=10
on=set()
for t in active:
    for k in range(TAIL+1): on.add(t+k)
on=[t for t in on if rows[0][0]<=t<=rows[-1][0]]
bursts=0; last=-99
for t in active:
    if t-last>TAIL: bursts+=1
    last=t
bs=open(f'/tmp/bt/exp/{L}.bs').read()
m=re.search(r',pwi,uid,([0-9.]+)',bs); pwi=m.group(1) if m else '?'
print(f"{L:22s} pol=[{open(f'/tmp/bt/exp/{L}.pol').read().strip()}]\n   pkts={pk} ({pk/dur:.2f}/s) bytes={by} | wakeups(>10s gap)={bursts} | est radio-on={len(on)}s/{dur}s = {100*len(on)/dur:.0f}% | ts pwi={pwi} mAh")
PY
