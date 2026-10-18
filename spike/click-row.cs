// Activating a row of a walk TableView.
//
// The obvious approach — LVM_SETITEMSTATE to select a row — does not work from
// another process. Its lParam is a pointer, Windows does not marshal pointers for
// LVM_* messages, and the target ListView ends up reading the *sender's* address
// space. It does not fail; it takes the application down.
//
// A double click carries coordinates instead, packed into lParam, and those
// marshal perfectly well.
using System;
using System.Runtime.InteropServices;
using System.Text;

public class Click
{
    public delegate bool EnumProc(IntPtr h, IntPtr l);

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }

    [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr p, EnumProc cb, IntPtr l);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetClassNameW(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h, uint msg, IntPtr wp, IntPtr lp);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out RECT r);

    const uint WM_LBUTTONDOWN = 0x0201;
    const uint WM_LBUTTONUP = 0x0202;
    const uint WM_LBUTTONDBLCLK = 0x0203;
    const int MK_LBUTTON = 0x0001;

    public static IntPtr FindClass(IntPtr parent, string cls)
    {
        IntPtr found = IntPtr.Zero;
        EnumChildWindows(parent, (h, l) =>
        {
            var sb = new StringBuilder(200);
            GetClassNameW(h, sb, 200);
            if (sb.ToString() == cls) { found = h; return false; }
            return true;
        }, IntPtr.Zero);
        return found;
    }

    static IntPtr Point(int x, int y) { return (IntPtr)((y << 16) | (x & 0xFFFF)); }

    // Double click at a client coordinate. row 0 is about twelve pixels down.
    public static void DoubleClickRow(IntPtr listview, int row)
    {
        int x = 60;
        int y = 12 + row * 19;
        var p = Point(x, y);

        SendMessage(listview, WM_LBUTTONDOWN, (IntPtr)MK_LBUTTON, p);
        SendMessage(listview, WM_LBUTTONUP, IntPtr.Zero, p);
        SendMessage(listview, WM_LBUTTONDBLCLK, (IntPtr)MK_LBUTTON, p);
        SendMessage(listview, WM_LBUTTONUP, IntPtr.Zero, p);
    }
}
