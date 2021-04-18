## FastXML Golang parser

This is a simple library that provides fast parsing of XMLs.

### Why is it fast?
Short answer: by using `unsafe` and skipping some XML features/validations.

Long answer: by having ability to only move through single buffer without need 
to read file and/or swap buffers it allows pointing strings to already known and static 
locations in that single buffer. In turn this means that no allocation is required
to move byte buffer into new location to create a string.

Also, by limiting number of (advanced) features provided by XML it is also greatly
improving simplicity and performance of parsing.

### Q&A
Q: Can it be used it in production?  
A: I wouldn't recommend it just yet. API and types also might change until stable release.

Q: If it is using `unsafe` does this mean that it can break something?  
A: No. It is using `unsafe` for only reason to point to specific memory for strings, nothing else.
Also, as time goes by this project will grow its set of test cases.

Q: Can it replace `encoding/xml`?  
A: Depends on the use case. If input document can fit in memory + it is known to be correct(valid XML) - then yes.  
Also keep in mind that having missing features in this parser will mean 
that more complex files(even if rules above being followed) - this parser may return incorrect results.

### Limitations
* This parser cannot be fully relied on to validate input XML.
* It may not implement full XML spec.
  For example, it does not support '!ELEMENT' and '!ENTITY' and other advanced features.
* Currently, file must be able loaded into memory fully to be able to parse it.
* Elements returned by the parser MUST NOT be hold onto. Only values of those elements can be stored.