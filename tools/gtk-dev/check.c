#include <gtk/gtk.h>

int main(int argc, char **argv) {
    if (!gtk_init_check(&argc, &argv)) {
        g_printerr("GTK display initialization failed\n");
        return 1;
    }
    GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_widget_destroy(window);
    g_print("GTK %u.%u.%u: window creation succeeded\n",
            gtk_get_major_version(), gtk_get_minor_version(), gtk_get_micro_version());
    return 0;
}
