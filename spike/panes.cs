// Finding the list view that actually has area.
//
// walk's TableView is a composite of two SysListView32 controls: a frozen one
// for columns pinned to the left, and a normal one for the rest. With no frozen
// columns the first is zero pixels wide, so a search that stops at the first
// match finds a window nobody can click — and every click lands nowhere, quietly.
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;

public class Panes
{
    public delegate bool EnumProc(IntPtr h, IntPtr l);

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }

    [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr p, EnumProc cb, IntPtr l);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetClassNameW(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);

    public static List<IntPtr> AllOfClass(IntPtr parent, string cls)
    {
        var found = new List<IntPtr>();
        EnumChildWindows(parent, (h, l) =>
        {
            var sb = new StringBuilder(200);
            GetClassNameW(h, sb, 200);
            if (sb.ToString() == cls) found.Add(h);
            return true;
        }, IntPtr.Zero);
        return found;
    }

    // The one an operator can actually see and click.
    public static IntPtr Widest(IntPtr parent, string cls)
    {
        IntPtr best = IntPtr.Zero;
        int widest = 0;

        foreach (var h in AllOfClass(parent, cls))
        {
            if (!IsWindowVisible(h)) continue;

            RECT r;
            GetWindowRect(h, out r);
            int width = r.Right - r.Left;
            if (width > widest) { widest = width; best = h; }
        }

        return best;
    }

    public static string Describe(IntPtr parent, string cls)
    {
        var lines = new List<string>();
        foreach (var h in AllOfClass(parent, cls))
        {
            RECT r;
            GetWindowRect(h, out r);
            lines.Add(string.Format("visible={0} ({1},{2})-({3},{4}) w={5}",
                IsWindowVisible(h), r.Left, r.Top, r.Right, r.Bottom, r.Right - r.Left));
        }
        return string.Join(" | ", lines);
    }
}
