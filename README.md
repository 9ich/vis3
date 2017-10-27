	NAME
		vis3 - visualize 3D polygons

	SYNOPSIS
		vis3 file
		[ prog ] | vis3

	DESCRIPTION
		Vis3 creates a window and then reads rendering commands from
		standard input, or from a file if a filename is specified in
		the first program argument. The view can be rotated using the
		mouse. The coordinate space is -1.0 to 1.0 in each axis.

	COMMANDS
		Commands build a scene, which persists until a clearscene command
		is read.

		Statements must be terminated by a semicolon, and a statement may
		span multiple lines.

		The commands are:

		clearscene
			removes all rendering commands from the scene
		color R G B A
			sets a new rendering color; components must be in range
			0.0 to 1.0
		bgcolor R G B A
			sets a new background color; components must be in range
			0.0 to 1.0
		thickness THICKNESS
			sets a new thickness for lines, in pixels
		pointsize SIZE
			sets a new size for points, in pixels
		point X Y Z
			places a point in the scene
		line AX AY AZ  BX BY BZ
			places the line AB in the scene
		poly X Y Z [ X Y Z ... ]
			places in the scene a convex polygon with any number
			of points
