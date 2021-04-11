/*
Package fastxml provides ability to quickly parse XML data.

Restriction for this parser is that the data should be able to fit into memory fully.
This restriction is currently based on the implementation and can be lifted in the future.

This parser does not fully implement XML, and probably never will.
But this is not the primary goal for this project. Performance is.

For example implementation of `!ENTITY` tag(if ever would be) will not fall under primary goal of this parser.
What this means is that it may allocate or be actually a performance bottleneck of this parser.
*/
package fastxml
