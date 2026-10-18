// Drives the FinGo entry form through the UI and reads the result back.
//
// Cells are found by where they are rather than by the order they were created
// in: a ComboBox contains an Edit of its own, so a flat list of "Edit" controls
// does not line up with the grid, and an index into it silently types an amount
// into the wrong box.
//
// Text goes in with WM_SETTEXT and is committed with WM_KILLFOCUS, which is what
// walk's LineEdit publishes editingFinished on — the same path a Tab takes.
using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;

public class Driver
{
    public delegate bool EnumProc(IntPtr h, IntPtr l);

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }

    [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr p, EnumProc cb, IntPtr l);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetClassNameW(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h, uint msg, IntPtr wp, IntPtr lp);
    [DllImport("user32.dll", CharSet = CharSet.Unicode, EntryPoint = "SendMessageW")]
    public static extern IntPtr SendText(IntPtr h, uint msg, IntPtr wp, string lp);
    [DllImport("user32.dll")] public static extern IntPtr GetParent(IntPtr h);
    [DllImport("user32.dll")] public static extern int GetDlgCtrlID(IntPtr h);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);

    const uint WM_SETTEXT = 0x000C;
    const uint WM_KILLFOCUS = 0x0008;
    const uint WM_COMMAND = 0x0111;
    const uint CB_SETCURSEL = 0x014E;
    const uint BM_CLICK = 0x00F5;
    const int CBN_SELCHANGE = 1;

    public class Control
    {
        public IntPtr Handle;
        public string Class;
        public RECT Rect;
        public string Text { get { return Driver.TextOf(Handle); } }
    }

    public static string TextOf(IntPtr h)
    {
        var sb = new StringBuilder(300);
        GetWindowTextW(h, sb, 300);
        return sb.ToString();
    }

    public static List<Control> Find(IntPtr parent, string cls)
    {
        var found = new List<Control>();
        EnumChildWindows(parent, (h, l) =>
        {
            var sb = new StringBuilder(200);
            GetClassNameW(h, sb, 200);
            if (sb.ToString() == cls && IsWindowVisible(h))
            {
                RECT r;
                GetWindowRect(h, out r);
                found.Add(new Control { Handle = h, Class = cls, Rect = r });
            }
            return true;
        }, IntPtr.Zero);
        return found;
    }

    // The form's rows, top to bottom, each holding its cells left to right.
    public static List<List<Control>> Rows(IntPtr parent)
    {
        var edits = Find(parent, "Edit").Where(c => c.Rect.Top > 150).ToList();
        var rows = new List<List<Control>>();

        foreach (var e in edits.OrderBy(c => c.Rect.Top).ThenBy(c => c.Rect.Left))
        {
            var row = rows.FirstOrDefault(r => Math.Abs(r[0].Rect.Top - e.Rect.Top) < 12);
            if (row == null) { row = new List<Control>(); rows.Add(row); }
            row.Add(e);
        }

        foreach (var r in rows) r.Sort((a, b) => a.Rect.Left.CompareTo(b.Rect.Left));
        return rows;
    }

    public static void Commit(IntPtr edit, string text)
    {
        SendText(edit, WM_SETTEXT, IntPtr.Zero, text);
        SendMessage(edit, WM_KILLFOCUS, IntPtr.Zero, IntPtr.Zero);
    }

    public static void Pick(IntPtr combo, int index)
    {
        SendMessage(combo, CB_SETCURSEL, (IntPtr)index, IntPtr.Zero);
        var parent = GetParent(combo);
        int id = GetDlgCtrlID(combo);
        int wparam = (id & 0xFFFF) | (CBN_SELCHANGE << 16);
        SendMessage(parent, WM_COMMAND, (IntPtr)wparam, combo);
    }

    public static IntPtr ButtonByText(IntPtr parent, string contains)
    {
        foreach (var b in Find(parent, "Button"))
            if (TextOf(b.Handle).Contains(contains)) return b.Handle;
        return IntPtr.Zero;
    }

    public static void Click(IntPtr button) { SendMessage(button, BM_CLICK, IntPtr.Zero, IntPtr.Zero); }
}
