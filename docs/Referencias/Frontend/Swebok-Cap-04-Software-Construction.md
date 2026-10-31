## CHAPTER 04

## Software Construction

## ACRONYMS

| API   | Application Programming Interface       |
|-------|-----------------------------------------|
| ASIC  | Application-Specific Integrated Circuit |
| BaaS  | Backend As A Service                    |
| CI    | Continuous Integration                  |
| COTS  | Commercial Off-The-Shelf                |
| CSS   | Cascading Style Sheets                  |
| DSL   | Domain-Specific Language                |
| DSP   | Digital Signal Processor                |
| ESB   | Enterprise Service Bus                  |
| FPGA  | Field Programmable Gate Array           |
| GPU   | Graphic Processing Unit                 |
| GUI   | Graphical User Interface                |
| HTML5 | Hypertext Markup Language Version 5     |
| IDE   | Integrated Development Environment      |
| JEE   | Jakarta Enterprise Edition              |
| MDA   | Model-Driven Architecture               |
| NPM   | Node Package Manager                    |
| OMG   | Object Management Group                 |
| PIM   | Platform Independent Model              |
| POSIX | Portable Operating System Interface     |
| PSM   | Platform-Specific Model                 |
| SDK   | Software Development Kit                |

| TDD     | Test-Driven Development      |
|---------|------------------------------|
| UML     | Unified Modeling Language    |
| WYSIWYG | What You See Is What You Get |

## INTRODUCTION

Software construction refers to the detailed creation and maintenance of software through coding, verification, unit testing, integration testing and debugging.

The software construction knowledge area (KA) is linked to all the other KAs, but it is most strongly linked to the Software Design and Software Testing KAs because the software construction process involves significant design and testing. The process uses the design output and provides an input to testing ("design" and "testing" in this case referring to the activities, not the KAs). Boundaries among design, construction and testing (if any) vary depending on the software life cycle processes used in a project.

Although some detailed design might be performed before construction, much design work is performed during construction. Thus, the Software Construction KA is closely linked to the Software Design KA.

Also, throughout construction, software engineers both unit-test and integration-test their work. Thus, the Software Construction KA is closely linked to the Software Testing KA as well.

The Software Construction KA is also related to configuration management, quality, project management and computing, and thus to the relevant KAs.

First, software construction typically pro- 1.1. Minimizing Complexity [1, c2, c3, duces the highest number of configuration c7-9, c24, c27, c28, c3, 1, c32, c34] items that need to be managed in a software project (e.g., source files, documentation, test All people have limited ability to hold comclosely linked to the Software Configuration Management KA.

cases). Thus, the Software Construction KA is plex structures and information in their working memories, especially over long periods. This greatly influences how people Second, while quality is important in all convey intent to computers and drives one the KAs, code is a software project's ultiof the key goals in software construction — mate deliverable, and code is produced to minimize complexity. The need to reduce during construction. Thus, the Software complexity applies to essentially every aspect Quality KA is closely linked to the Software of software construction and is particularly Construction KA. critical to testing software constructions.

Third, while project management involves various software development tasks, software construction typically produces the most deliverables of a software project. Thus, the Software Construction KA is closely linked to the Software Engineering Management KA.

Fourth, since software construction requires knowledge of algorithms and coding practices, this KA is closely related to the Computing Foundations KA, which concerns the computer science foundations supporting software product design and construction.

## BREAKDOWN OFTOPICS FOR SOFTWARE CONSTRUCTION

Several types of complexity can affect software construction. Tools can be used to manage different aspects of the complexity of software components and their construction. For example, cyclomatic complexity is a static analysis measure of how difficult code is to test and understand. The tool, developed by Thomas J. McCabe, Sr., in 1976, calculates the number of linearly independent paths through a program's source code. Ideally, there should be at least that number of test cases. Other examples are tools like Make, which can build an application, or integrated development environments (IDEs) for entering, editing and compiling code. These tools help manage the complexity of the construction process.

In software construction, reduced comThe breakdown of topics for the Software plexity is achieved by creating simple and Architecture KA is shown in Figure 4-1. readable code rather than clever code. This is accomplished by using standards (see section 1. Software Construction Fundamentals 3.1.5, Standards in Construction), modular design (see section 3.1, Construction Design) Software construction fundamentals include and numerous other specific techniques (see the following: section 3.3, Coding). Construction-focused quality techniques also support this (see sec· Minimizing complexity tion 3.6, Construction Quality).

- Anticipating and embracing change
- Constructing for verification
- Reusing assets
- Applying standards in construction

<!-- formula-not-decoded -->

<!-- formula-not-decoded -->

The first four concepts apply to design as Most software changes over time, and well as to construction. The following sections anticipating change drives many aspects of define these concepts and describe how they software construction; changes in the enviapply to construction. ronments in which software operates also affect software in diverse ways. Anticipating necessary changes can be difficult, so softchange helps software engineers build ware engineers should be careful to build extensible software, enhancing a software flexibility and adaptability into the softproduct without disrupting the underlying ware to incorporate changes with less diffistructure. Anticipating change is supported culty. These software teams should embrace by many specific techniques (see section change by adopting agile development, 3.3, Coding). practicing DevOps, and by adopting conMoreover, today's business environments tinuous delivery and deployment practices.

Figure 4.1. Breakdown of Topics for the Software Construction KA

<!-- image -->

require many organizations to deliver and Such practices align the software develop-o  n  n   y  o or o and more reliably. Anticipating specific, lutionary environment.

<!-- formula-not-decoded -->

Constructing for verification builds software in such a way that faults can be readily found by the software engineers writing the software as well as by the testers and users during independent testing and operational activities. Specific techniques that support constructing for verification include following coding standards to support code reviews and unit testing, organizing code to support automated testing, restricting the use of complex or difficult-to-understand language structures, and recording software behaviors with logs.

<!-- formula-not-decoded -->

Reuse means using existing assets to solve different problems. In software construction, typical assets that are reused include frameworks, libraries, modules, components, source code and commercial off-the-shelf (COTS) assets. Reuse has two closely related facets: construction for reuse and construction with reuse. Use of internal standards: Standards may The former means creating reusable software also be created on an organizational basis at the assets, whereas the latter means reusing softcorporate level or for use on specific projects. ware assets to construct a new solution. Reuse These standards support coordinating group often transcends project boundaries, which activities, minimizing complexity, anticipating means reused assets can be constructed in change and constructing for verification. other projects or organizations.

- Coding standards (e.g., standards for naming conventions, layout and indentation)
- Exception handling policies (e.g., standards for the information included in exceptions and the way how exceptions are handled after catching)
- Platforms (e.g., interface standards for operating system calls)
- Tools (e.g., diagrammatic standards for notations like UML - Unified Modeling Language)

Use of external standards: Construction depends on external standards for construction languages, construction tools, technical interfaces and interactions between the Software [2-c15] Construction KA and other KAs. Standards come from numerous sources, including hardware and software interface specifications (e.g., Object Management Group (OMG)) and international organizations (e.g., the Institute of Electrical and Electronics Engineers (IEEE), the International Organization for Standardization (ISO)).

## 2. Managing Construction

<!-- formula-not-decoded -->

<!-- formula-not-decoded -->

Numerous models have been created to tives. Specifically, the choices of allowable describe the development of software; some emphasize construction more than others.

Some models are more linear from the construction viewpoint, such as the waterfall Standards that directly affect construction and staged-delivery life cycle models. These issues include the following: models treat construction as an activity that occurs only after the completion of significant · Communication methods (e.g., standards prerequisite work, including detailed requirefor document formats and content) ments work, extensive design work and detailed · Programming languages (e.g., standards planning. The more linear approaches emphafor languages like Java and C++) size the activities that precede construction

## 1.5. Applying Standards in Construction [1-c4]

Applying external or internal development standards during construction helps achieve a project's efficiency, quality and cost objecprogramming language subsets and usage standards are important aids in achieving higher security.

(requirements and design) and create more Construction planning also defines the distinct separations between activities. In order in which components are created and these models, construction's main emphasis integrated, the integration strategy (for might be coding. example, phased or incremental integration), the software quality management processes, the allocation of task assignments to specific software engineers, and other tasks, according to the chosen method.

combination of activities as construction (see facts can be measured, including code develthe Software Engineering Management and oped, modified, reused, and destroyed; code complexity; code inspection statistics; fault-fix 1 and fault-find rates; effort; and scheduling. These measurements can be useful for managing construction, ensuring quality during construction and improving the construction process, among other uses (see the Software Engineering Process KA for more on measurement).

Other models, such as evolutionary prototyping and agile development, are more iterative. These approaches treat construction as an activity that occurs concurrently with or overlaps other software development activities (including requirements, design and plan- 2.3. Construction Measurement [1-c25, c28] ning). These approaches mix design, coding and testing activities, and they often treat the Numerous construction activities and artiSoftware Process KAs).

The practices of continuous delivery and deployment further mix coding, testing, delivery and deployment activities. In these practices, software updates made during construction activities are continuously delivered and deployed into the production environment. The whole process is fully automated by a deployment pipeline that consists of various testing and deployment activities.

<!-- formula-not-decoded -->

Consequently, what is considered construction depends on the life cycle model Software products often heavily rely on depenused. In general, software construction is mostly coding and debugging, but it also involves construction planning, detailed allow developers to reuse common functionaldesign, unit testing, integration testing and other activities.

<!-- formula-not-decoded -->

dencies, including internal and external (commercial or open-source) dependencies, which ities instead of reinventing the wheel and substantially improve developers' productivity. In addition, package managers (e.g., Maven in [1-c3, c4,Java and NPM in JavaScript) are widely used to automate the process of installing, upgrading, configuring and removing dependencies.

The choice of construction method is a key The direct and indirect dependencies of aspect of the construction planning activity. This software products constitute a dependency choice affects the extent to which construcsupply chain network. Any dependency tion prerequisites are performed, the order in in the supply chain network can introwhich they are performed and the degree to duce potential risk to software products and which they should be completed before conshould be managed by developers or tools. struction work begins. Unnecessary dependencies should be avoided The approach to construction affects the to improve build efficiency. License conflicts project team's ability to reduce complexity, between dependencies and software prodanticipate change and construct for verificaucts should be avoided to reduce legal risk. tion. Each objective may also be addressed at Propagation of dependencies' defects or vulthe process, requirements and design levels, nerabilities into software products should but the choice of construction method will be avoided to improve the quality of softinfluence them. ware products. Regulations and monitoring mechanisms should be developed to preThe simplest construction language is a vent developers from introducing untrusted configuration language, in which software external dependencies. engineers choose from a limited set of predefined options to create new or custom 3. Practical Considerations software installations. The text-based configuration files used in both the Windows and Construction is an activity in which the soft- Unix operating systems are examples of this, and some program generators' menu-style selection lists constitute another example of a ware engineer often has to deal with sometimes chaotic, changing and even conflicting real-world constraints. Because of real-world configuration language. constraints, practical considerations drive construction more than some other KAs, and cations from elements in toolkits (integrated software engineering is perhaps most craft- sets of application-specific reusable parts); like in the construction activities compared they are more complex than configuration with other activities.

<!-- formula-not-decoded -->

Toolkit languages are used to build applilanguages. Toolkit languages may be explicitly defined as application programming languages, or the applications might be implied by a toolkit's set of interfaces.

Scripting languages are commonly used activity to construction, whereas others allo- application programming languages. In some c  lor sas ss tts o e c osd    atcb

Programming languages are the most flexstruction level, and that design work is dic- ible construction languages. They also contain tated by constraints imposed by the real-world the least amount of information about specific application areas and development processes. Just as construction workers building a Therefore, they require the most training and physical structure must make small modifiskill to use effectively. The choice of programcations for unanticipated gaps in the builder's ming language can greatly affect the likeplans, software construction workers must lihood of vulnerabilities being introduced make small or large modifications to flesh out during coding (e.g., unsafe use of C and C++ software design details during construction. library functions is questionable from a security viewpoint).

Some projects allocate considerable design design. Regardless of the exact allocation, files or macros. some detailed design work occurs at the conproblem the software addresses.

The details of the design activity at the construction level are essentially the same as described in the Software Design KA, but they are applied at a smaller scale to algorithms, data structures and interfaces.

<!-- formula-not-decoded -->

Three general notations are used for programming languages:

- Linguistic (e.g., C/C++, Java)
- Formal (e.g., Event-B)
- Visual (e.g., MATLAB)

Construction languages include all forms Linguistic notations are distinguished in of communication by which a human can particular by the use of textual strings to repspecify an executable solution to a problem. resent complex software constructions. The Consequently, construction languages and combination of textual strings in patterns may their implementations (e.g., compilers) can have a sentence-like syntax. Properly used, affect software quality attributes such as pereach string should have a strong semantic formance, reliability and portability. As a connotation providing an immediate intuiresult, they can seriously contribute to secutive understanding of what happens when the rity vulnerabilities. software construction is executed.

Formal notations rely less on intuitive, everyday meanings of words and text strings and more on definitions backed by precise, unambiguous and formal (or mathematical) definitions. Formal construction notations and methods are at the semantic base of most system programming notations, where accuracy, time behavior and testability are more important than ease of mapping into natural language. Formal constructions also use precisely defined ways of combining symbols that avoid the ambiguity of many natural language constructions.

variables, named constants and other similar entities

- Use of control structures
- Handling of error conditions — both anticipated and exceptional (e.g., input of bad data)
- Prevention of code-level security breaches (e.g., buffer overflows or array index bounds)
- Resource use through use of exclusion mechanisms and discipline in accessing serially reusable resources, including threads and database locks

Visual notations rely much less on the Source code organization into statetextual notations of linguistic and formal ments, routines, classes, packages or construction and more on direct visual interother structures pretation and placement of visual entities that Code documentation represent the underlying software. Visual Code tuning construction is somewhat limited by the difficulty of making "complex" statements using 3.4. Construction Testing [1-c22, c23, 2-c8] only the arrangement of icons on a display. However, these icons can be powerful tools Construction involves two forms of testing, in cases where the primary programming task which are often performed by the software is to build and "adjust" a visual interface to a engineer who wrote the code: unit testing and program, the detailed behavior of which has integration testing. an underlying definition.

cific applications. Unlike a general-purpose programming language, such as C/C++ or Java, a DSL is designed for the applicaTherefore, a DSL usually can be defined before code is written. based on a higher level of abstraction of the Construction testing typically involves target domain and can be optimized for a a subset of the various types of testing, specific class of problems. Furthermore, A described in the Software Testing KA. For DSL usually can be expressed by visual nota- instance, construction testing does not typtions defined by domain-specific concepts ically include system testing, alpha testing, and rules. beta testing, stress testing, configuration testing, usability testing, or other more speConstruction testing aims to reduce Nowadays, domain-specific languages the gap between when faults are inserted (DSLs) are widely used to build domain-spe- into the code and when those faults are detected, thereby reducing the cost incurred to fix them. In some instances, test cases are written after the code has been written. In tion construction of a particular domain. other instances, test cases might be created

<!-- formula-not-decoded -->

Two standards have been published on The following considerations apply to the construction testing: IEEE Standard 829software construction coding activity: 2008, "IEEE Standard for Software Test Documentation," and "IEEE Standard for

See sections 2.1.1 and 2.1.2 in the Software Testing KA for more specialized refer-

- Use of classes, enumerated types, ence material.
- Techniques for creating understandable Software Unit Testing." source code, including naming conventions and source code layout

## 3.5. Reuse in Construction [2-c15,c16]

Reuse in construction includes both construction for reuse and construction with reuse.

Construction for reuse creates software with the potential to be reused in the future locally integrated. Nowadays, cloud services for the present project or for other projects with a broad-based, multisystem perspective. Construction for reuse is usually based on variability analysis and design. To avoid the problem of code clones, developers should encapsulate reusable code fragments into well-structured libraries or components.

The tasks related to software construction for reuse during coding and testing are as follows:

- Reporting reuse information on new code, test procedures or test data

The forms of reusable software assets are not limited to software artifacts that must be that provide various services through online interfaces such as RESTful application programming interfaces (APIs) are widely used in applications. In the new cloud service model BaaS (backend as a service), applications delegate their backend implementations to cloud service providers — for example, utilities such as authentication, messaging and storage are usually provided by cloud providers.

Reuse is best practiced systematically, according to a well-defined, repeatable pro· Variability implementation with mechcess. Systematic reuse can enable signifianisms such as parameterization, condicant software productivity, quality and cost tional compilation and design patterns improvements. Systematic reuse is supported Variability encapsulation to make by methodologies such as software product the software assets easy to configure line engineering and various software frameand customize works and platforms. Widely used frameworks Testing the variability provided by the such as Spring provide reusable infrastrucreusable software assets tures for enterprise applications so softDescription and publication of reusable ware teams can focus on application-specific software assets business logic. Commercial platforms provide various reusable frameworks, libraries, components and tools to support application development to build their ecosystems.

open-source libraries. In addition, reused ments and design activities, faults introduced and off-the-shelf software often have the during construction can cause serious quality problems (e.g., security vulnerabilities). These newly developed software (e.g., security level include not only faults in security functionality but also faults elsewhere that allow bypassing of the security functionality or create other security weaknesses or violations.

Construction with reuse means creating new software by reusing existing software assets. The most popular reuse method is to reuse code from the libraries provided by 3.6. Construction Quality [1-c8, c20-c25, the language, platform, tools or an organi2-c8, c24] zational repository. Aside from these, many applications developed today use third-party In addition to faults occurring during requiresame (or better) quality requirements as requirements).

The tasks related to software construction with reuse during coding and testing are as follows:

- Selecting reusable units, databases, test procedures or test data
- Evaluating code or test reusability
- Integrating reusable software assets into the current software

Numerous techniques exist to ensure the quality of code as it is constructed. The primary techniques used to ensure construction quality are:

- Unit testing and integration testing (see section 3.4, Construction Testing)

· Test-first development (see section 6.1.2 Programs can be integrated by means in the Software Testing KA) of either the phased or the incremental · Use of assertions and defensive approach. Phased integration, also called big programming bang integration, entails delaying the inte· Debugging gration of component software parts until · Inspections all parts intended for release in a version are · Technical reviews, including securicomplete. Incremental integration is thought ty-oriented reviews (see section 2.3 in the to offer many advantages over the traditional Software Quality KA) phased integration (e.g., easier error locaStatic analysis (see section 2.2.1 of the tion, improved progress monitoring, earlier Software Quality KA) product delivery and improved customer relations). In incremental integration, the develThe specific technique or techniques opers write and test a program in small pieces selected depend on the software constructed and then combine the pieces one at a time. and on the skill set of the software engiAdditional test infrastructure, including, for neers performing the construction activities. example, stubs, drivers and mock objects, is Programmers should know good practices and usually needed to enable incremental integracommon vulnerabilities (e.g., from widely rection. In addition, by building and integrating ognized lists about common vulnerabilities). one unit at a time (e.g., a class or component), Automated static code analysis for security the construction process can provide early weaknesses is available for several common feedback to developers and customers. programming languages.

er t ts tts  s ts   t tt  ty their focus. These activities focus on arti- leading to multiple integrations per day. CI ste t ud    ui isd uds  u    u  sids as detailed design — as opposed to other and tests each integration to detect errors and artifacts that are less directly connected to the code, such as requirements, high-level designs and plans.

Today, continuous integration (CI) has Construction quality activities are dif- been widely adopted in practice. A software provide fast feedback.

<!-- formula-not-decoded -->

<!-- formula-not-decoded -->

Some applications, such as mobile applicaDuring construction, a key activity is inte- tions, heavily rely on specific platforms (e.g., grating individually constructed routines, Apple, Android), which usually include classes, components and subsystems into a operating systems, development frameworks single system. In addition, a particular soft- and APIs. To support multiple platforms, ware system may need to be integrated with the developers need to develop and build an other software or hardware systems. application separately for each target platform using the corresponding program language and software development kit (SDK). However, multi-platform development in this way requires more time and cost and might cause different user experiences between different implementations.

Cross-platform development allows the developers to develop an application using a universal language and export it to various platforms. This usually can be done in two

Concerns related to construction integration include planning the sequence in which components are integrated, identifying what hardware is needed, creating scaffolding to support interim versions of the software, determining the degree of testing and quality work performed on components before they are integrated, and determining points in the project at which interim versions of the software are tested.

ways for mobile applications. One way is to approach has been widely used, which emphagenerate native applications using tools that sizes designing and building the APIs of an application first. In practice, the API-first approach is usually accomplished by using an API description language to establish a contract for how the API is supposed to behave.

## 4.2. Object-Oriented Runtime Issues [1-c6, c7]

Object-oriented languages support runFor applications that are not developed in time mechanisms, including polymorphism ; and reflection. These runtime mechanisms increase the flexibility and adaptability of

can compile the universal language into platform-specific formats. The other is to develop hybrid applications that combine web applications developed using languages like hypertext markup language version 5 (HTML5) and cascading style sheets (CSS) and native containers or wrappers for various operations systems.

this way, developers may consider migrating the applications from one platform to another. The migration usually involves translation of object-oriented programs. different programming languages and platform-specific APIs and can be partially automated by tools.

## 4. Construction Technologies

<!-- formula-not-decoded -->

Polymorphism is a language's ability to support general operations without knowing until runtime what kind of concrete objects the software will include. Because the program does not know the types of the objects in advance, the exact behavior is determined [5-c7] at runtime (called dynamic binding).

Reflection is a program's ability to observe An API is a set of signatures that are exported and modify its structure and behavior at runand available to the users of a library or a time. For example, reflection allows inspection framework to write their applications. Besides of classes, interfaces, fields and methods at -   s   om s ss os m s som statements about the program's effects and/or pile time. It also allows instantiation of new behaviors (i.e., its semantics). objects at runtime and invocation of methods using parameterized class and method names.

## 4.3. Parameterization, Templates, and Generics [6-c1]

Parameterized types, also known as generics an API should be straightforward and stable, (Ada, Java, Eiffel) and templates (C++), enable a type or class definition without specifying all the other types used. The unspecified types API use involves selecting, learning, are supplied as parameters at the point of use. testing, integrating and possibly extending Parameterized types provide a third way APIs provided by a library or framework (see (besides class inheritance and object composection 3.5, Reuse in Construction). sition) to compose behaviors in object-oriented software.

<!-- formula-not-decoded -->

An assertion is an executable predicate placed popular languages such as Java, JavaScript, in a program — usually a routine or macro — Python, etc. At the same time, the API-first that performs runtime checks of the program.

API design should make the API easy to learn and memorize, lead to readable code, be difficult to misuse, be easy to extend, be complete, and maintain backward compatibility. As the APIs usually outlast their implementations for a widely used library or framework, to facilitate client application development and maintenance.

For online interfaces such as RESTful APIs, open standards such as OpenAPI play an important role. OpenAPI defines a standard, language-agnostic interface to HTTP APIs and supports the automatic generation of server-side and client-side code, covering Assertions are especially useful in high-reli- all information that led to the exception, ability programs. They enable programmers to avoiding empty catch blocks, knowing the more quickly flush out mismatched interface exceptions the library code throws, perhaps assumptions, errors that creep in when code is building a centralized exception reporter, modified, and other problems. Assertions are and standardizing the program's use of typically compiled into the code at developexceptions. ment time and are later compiled out of the code so they don't degrade the performance.

Design by contract is a development approach in which preconditions and postconditions are included for each routine. When preconditions and postconditions are used, each routine or class is said to form a contract with specifies the semantics of a routine and thus helps clarify its behavior. Design by contract is thought to improve the quality of software construction.

Fault tolerance is a collection of techniques that increase software reliability by detecting errors and then recovering from them or containing their effects if recovery is not possible. The most common fault tolerance strategies include backing up and retrying, using auxiliary code and voting algorithms, and the rest of the program. A contract precisely replacing an erroneous value with a phony value that will have a benign effect.

<!-- formula-not-decoded -->

Defensive programming means to protect a Executable models abstract away the details of routine from being broken by invalid inputs. specific programming languages and deciCommon ways to handle invalid inputs sions about the software's organization. include checking the values of all the input Different from traditional software models, parameters and deciding how to handle bad a specification built in an executable modinputs. Assertions are often used in defensive eling language like xUML (executable UML) programming to check input values. can be deployed in various software environments without change. Furthermore, an exe4.5. Error Handling, Exception Handling, and cutable-model compiler (transformer) can Fault Tolerance [1-c8, c9] turn an executable model into an implementation using a set of decisions about the target hardware and software environment. Thus, constructing executable models is a way of constructing executable software.

Executable models are one foundation supporting the model-driven architecture (MDA) initiative of the OMG. An executable model is a way to specify a platform-independent model (PIM); a PIM is a model of a solution to a problem that does not rely on any implementation technologies. Then a platform-specific model (PSM), which is a model that contains the details of the implementation, can be produced by weaving together the PIM and the platform on which it relies.

How errors are handled affects software's ability to meet requirements related to correctness, robustness and other nonfunctional attributes. Assertions are sometimes used to check for errors. Other error-handling techniques — such as returning a neutral value, substituting the next piece of valid data, logging a warning message, returning an error code or shutting down the software — are also used.

Exceptions are used to detect and process errors or exceptional events. The basic structure of an exception is as follows: A routine uses throw to throw a detected exception, and an exception-handling block will catch the exception in a try-catch block. The trycatch block may process the erroneous condi4.7. State-Based and Table-Driven tion or return control to the calling routine. Construction Techniques [1-c18] Exception-handling policies should be carefully designed following common principles, State-based programming, or automata-based

such as including in the exception message programming, is a programming technology that uses finite-state machines to describe 4.9. Grammar-Based Input Processing [1,8] program behaviors. A state machine's transition graphs are used in all stages of software Grammar-based input processing involves development (specification, implementation, syntax analysis, or parsing, of the input token debugging and documentation). The main stream. It involves the creation of a data strucidea is to construct computer programs in the ture (called a parse tree or syntax tree) represame way technological processes are autosenting the input data. The inorder traversal mated. State-based programming is usually of the parse tree usually gives the exprescombined with object-oriented programming, sion just parsed. Next, the parser checks the forming a new composite approach called 1 symbol table for programmer-defined varistate-based, object-oriented programming. ables that populate the tree. After building A table-driven method is a schema that the parse tree, the program uses it as an input uses tables to display information rather to the computational processes. than convey information with logic statements (such as if and case). When used in 4.10.Concurrency Primitives [9-c6] appropriate circumstances, table-driven code is simpler than complicated logic and A synchronization primitive is a programming easier to modify. When using table-driven abstraction provided by a programming lanmethods, the programmer addresses two guage or the operating system that facilitates issues: what information to store in the table concurrency and synchronization. Wellor tables and how to efficiently access infor- known concurrency primitives include semamation in the table. phores, monitors and mutexes.

A semaphore is a protected variable or 4.8. Runtime Configuration and abstract data type that provides a simple Internationalization [1-c3,c10] but useful abstraction for controlling access to a common resource by multiple processes To achieve more flexibility, a program is or threads in a concurrent programming environment.

A monitor is an abstract data type that presents a set of programmer-defined operations executed with mutual exclusion. A monitor contains the declaration of shared variables and procedures or functions that operate on Internationalization is the technical activity those variables. The monitor construct ensures

often constructed to support its variables' late binding time. For example, runtime configuration binds variable values and program settings when the program is running, usually by updating and reading configuration files in a just-in-time mode.

of preparing a program, usually interactive that only one process at a time is active in software, to support multiple locales. The cor- the monitor. responding activity, localization, modifies a A mutex (mutual exclusion) is a synchroprogram to support a specific local language. nization primitive that grants exclusive access Interactive software may contain dozens or to a shared resource by only one process or hundreds of prompts, status displays, help thread at a time. messages, error messages and so on. The design and construction processes should 4.11.Middleware [5-c1,8-c8] accommodate string and character set issues, including which character set is used, what Middleware is a broad classification for softkinds of strings are used, how to maintain the ware that provides services above the operating strings without changing the code and how to system layer yet below the application protranslate the strings into different languages gram layer. Middleware can provide runtime with minimal impact on the processing code containers for software components to provide and the user interface. message passing, persistence and a transparent location across a network. Middleware can Digital Signal Processors (DSPs), microbe viewed as a connector between the comcontrollers and peripheral processors. These ponents using the middleware. Modern mescomputational units are independently consage-oriented middleware usually provides an trolled and communicate with one another. enterprise service bus (ESB) that supports serEmbedded systems are typically heterogevice-oriented interaction and communication neous systems. among multiple software applications.

## 4.12.Construction Methods for Distributed and Cloud-Based Software [2-c17, c18,9-c2]

The design of heterogeneous systems may require combining several specification languages to design different system parts (hardware/software codesign). The key issues include multilanguage validation, co-simulation and

A distributed system is a collection of physically interfacing. separate, possibly heterogeneous computer systems networked to provide the users with access to the resources the system maintains. The construction of distributed software is distinguished from traditional software construction by issues such as parallelism, communication and fault tolerance.

During the hardware/software codesign, 1 software and virtual hardware development proceed concurrently through stepwise decomposition. The hardware part is usually simulated in field programmable gate arrays (FPGAs) or application-specific integrated circuits (ASICs). The software part is translated into a

<!-- formula-not-decoded -->

Code efficiency — determined by architecture, detailed design decisions, and data structure and algorithm selection — influences execution speed and size. Performance analysis investigates a program's behavior using information gathered as the program executes to

Distributed programming typically falls low-level programming language. into several basic architectural categories: client-server, three-tier architecture, n-tier architecture, distributed objects, loose coupling or tight coupling (see section 5.6 in the Computing Foundations KA and section 2.2 in the Software Architecture KA).

Nowadays, more applications are migrated to the cloud. Cloud-based software often adopts microservice architecture and container-based deployment. In addition to traditional distributed software issues, cloud-based soft- identify possible hot spots in the program to ware developers also need to consider cloud be improved. infrastructure issues such as use of an API gateway, service registration and discovery.

Code tuning, which improves performance at the code level, modifies code to make it run Distributed systems based on n-tier/ser- more efficiently. Code tuning usually involves vice-oriented architectures usually rely on only small changes that affect a single class, ACID distributed transactions for the imple- a single routine or, more commonly, a few mentation of transactions involving multiple lines of code. A rich set of code tuning techdistributed components. In contrast, cloud- niques is available, including those for tuning based microservices cannot enforce distributed logic expressions, loops, data transformations, transactions consistency, and use some form of expressions and routines. Using a low-level SAGA-based eventual consistency, initially language is another common technique for intended for long-running transactions. improving hot spots in a program.

s[4-c, 8-c10,9-c1]

## 4'.tg s [1]  ts t uts

Heterogeneous systems consist of various special- Platform standards enable programmers to ized computational units of different types, develop portable applications that can be exesuch as Graphic Processing Units (GPUs) and cuted in compatible environments without changes. Platform standards usually involve 5. Software Construction Tools standard services and APIs that compatible platform implementations must use. 5.1. Development Environments [1-c30] Typical examples of platform standards are Jakarta Enterprise Edition (JEE); the porA development environment, or integrated table operating system interface (POSIX) development environment (IDE), provides standard for operating systems, which repcomprehensive facilities to programmers for resents a set of standards implemented prisoftware construction by integrating a set of marily for Unix-based operating systems; development tools. The programmers' choice and HTML5, which defines the standards of development environment can affect softfor developing web applications that can ware construction efficiency and quality. run on different environments (e.g., Apple iOS, Android).

Test-first programming (also known as TDD - Test-Driven Development) is a popular devel- forms, and support for refactoring. opment style in which test cases are written Nowadays, cloud-based development envibefore any code. These test cases, when applied ronments are available in public or private to the current code base, will fail. Code is then cloud services. These environments can prowritten that will allow the test cases to pass. vide all the features of modern IDEs and At that time, the new code and associated even more (e.g., containerized building and parts of the project can be refactored and optideployment), powered by the cloud. mized. Test-first programming can usually detect defects earlier and correct them more Furthermore, writing test cases first forces programmers to think about requirements and design before coding, thus exposing requirements and design problems sooner.

Besides basic code editing functions, modern IDEs often offer other features, like compilation and error detection within the 4.16. Test-First Programming [1-c22, 2-c8] editor, integration with source code control, build/test/debugging tools, condensed or outline views of programs, automated code transMoreover, modern IDEs are often equipped with AI-assisted programming which is ea  s o t  g s   a cgs Language Models (LLMs). With the support a programmer can define a function in pseudocode comments or outline its implementation as a prompt for an LLM to generate or complete the code. The programmer lets the 4.17. Feedback Loop for Construction LLM complete many of the details, but still [3-c3,c16] reviews the generated code and integrates it into their project.

Early and continuous feedback for the construction activity is one of the most 5.2. Visual Programming and Low-Code/Zeroimportant advantages of agile development Code Platforms [1-c30] and DevOps. Agile development provides early feedback for construction through freVisual programming allows users to create proquent iterations in the development process. grams by manipulating visual program eleDevOps provides even faster feedback from ments graphically. As a visual programming the operation, allowing the developers to tool, a GUI (graphical user interface) builder learn how well their code performs in proenables the developer to create and mainduction environments. This fast feedback is tain GUIs in a WYSIWYG (what you see achieved through techniques and practices is what you get) mode. A GUI builder usuin the DevOps pipeline, such as automated ally includes a visual editor that enables the building and testing, canary release, and developer to design forms and windows and A/B testing. manage the layout of the widgets with drag, drop and parameter setting features. Some is often automated. Developers can use unit GUI builders can automatically generate the testing tools and frameworks to extend and source code corresponding to the visual GUI create an automated testing environment. For design. Because GUI applications usually example, the developer can code criteria into follow the event-driven style (in which events the test with unit testing tools and frameand event handling determine the program works to verify the unit's correctness under flow), GUI builder tools usually provide code various data sets. Each test is implemented generation assistants, which automate the as an object, and a test runner runs the tests. most repetitive tasks required for event hanFailed test cases are automatically flagged and dling. The supporting code connects widgets reported during the test execution. with the outgoing and incoming events that trigger the functions providing the application 5.4. Profiling, Performance Analysis, logic. Some modern IDEs provide integrated and Slicing Tools [1-c25,c26] GUI builders or GUI builder plug-ins. There are also many stand-alone GUI builders.

Program slicing involves computing the set of program statements (i.e., the program slice) that might affect the values of specified variables at some point of interest, which is called 5.3. Unit Testing Tools [1-c22, 2-c8]a slicing criterion. Program slicing can be used for locating error sources, program understanding and optimization analysis. Program slicing tools compute program slices for varrately testable software elements (for example, ious programming languages using static or

Performance analysis tools are usually used to Visual programming and other rapid applisupport code tuning. The most common percation development tools have evolved into formance analysis tools are profling tools. An low-code/zero-code platforms. These platforms execution profiling tool monitors the code allow developers to build complete applicawhile it runs and records how often each tions visually through a drag-and-drop interstatement is executed or how much time the face and with minimal hand-coding. They program spends on each statement or exeare usually based on the principles of modcution path. Profiling the code while it runs el-driven design, visual programming and gives insight into how the program works, code generation. The difference between lowwhere the hot spots are and where the develcode development and zero-code development opers should focus the code tuning efforts. lies in hand-coding; the former requires a little hand-coding, whereas the latter requires practically none.

Unit testing verifies the functioning of software modules in isolation from other sepaclasses, routines, components). Unit testing dynamic analysis methods.

## MATRIX OFTOPICS VS. REFERENCE MATERIAL

|                                       | McConnell, 2004 [1]   | Sommerville, 2016 [2]   | Kim et al., 2021 [3]   | et al., 2013 [4] Heitkötter   | et al., 2010 [5] Clements   | Gamma et al. 1994 [6]   | Mellor and Balcer, 2002 [7]   | Null and Lobur, 2018 [8]   | et al., 2008 [9] Silberschatz   |
|---------------------------------------|-----------------------|-------------------------|------------------------|-------------------------------|-----------------------------|-------------------------|-------------------------------|----------------------------|---------------------------------|
| 1. Software Construction Fundamentals |                       |                         |                        |                               |                             |                         |                               |                            |                                 |

| 1.1. Minimizing Complexity                    | c2, c3, c7-c9, c24, c27, c28, c31, c32, c34   |           |     |    |    |
|-----------------------------------------------|-----------------------------------------------|-----------|-----|----|----|
| 1.2. Anticipating and Embracing Change        | c3-c5, c24, c31, c32, c34                     | c1,c3, 60 | c1  |    |    |
| 1.3. Constructing for Verification            | c8, c20-c23, c31, c34                         |           |     |    |    |
| 1.4. Reuse                                    |                                               | c15       |     |    |    |
| 1.5. Standards in Construction                | c4                                            |           |     |    |    |
| 2. Managing Construction                      |                                               |           |     |    |    |
| 2.1. Construction in Life Cycle Models        | c2, c3, c27, c29                              | c3, c7    | c1  |    |    |
| 2.2. Construction Planning                    | c3, c4, c21, c27-c29                          |           |     |    |    |
| 2.3. Construction Measurement                 | c25, c28                                      |           |     |    |    |
| 2.4. Managing Dependencies                    |                                               | c25       |     |    |    |
| 3. Practical Considerations                   |                                               |           |     |    |    |
| 3.1. Construction Design                      | c3, c5, c24                                   | c7        |     |    |    |
| 3.2. Construction Languages                   | 04                                            |           |     |    |    |
| 3.3. Coding                                   | c5-c19, c25-c26                               |           |     |    |    |
| 3.4. Construction Testing                     | c22, c23                                      | c8        |     |    |    |
| 3.5. Reuse in Construction                    |                                               | c15, c16  |     |    |    |
| 3.6. Construction Quality                     | c8, c20-c25                                   | c8, c24   |     |    |    |
| 3.7. Integration                              | c29                                           | c8        | c11 |    |    |
| 3.8. Cross-Platform Development and Migration |                                               |           |     | C  |    |
| 4. Construction Technologies                  |                                               |           |     |    |    |
| 4.1. API Design and Use                       |                                               |           |     |    | c7 |
| 4.2. Object-Oriented Runtime Issues           | c6,c7                                         |           |     |    |    |

| 4.3. Parameterization, Templates and Generics                       |          |          |         |    |    | c1   |     |    |
|---------------------------------------------------------------------|----------|----------|---------|----|----|------|-----|----|
| 4.4. Assertions, Design by Contract and Defensive Programming       | c8, c9   |          |         |    |    |      |     |    |
| 4.5. Error Handling, Exception Handling and Fault Tolerance         | c3, c8   |          |         |    |    |      |     |    |
| 4.6. Executable Models                                              |          |          |         |    |    |      |     |    |
| 4.7. State-Based and Table-Driven Construction Techniques           | c18      |          |         |    |    |      |     |    |
| 4.8. Runtime Configuration and Internationalization                 | c3, c10  |          |         |    |    |      |     |    |
| 4.9. Grammar-Based Input Processing                                 | c5       |          |         |    |    |      | c8  |    |
| 4.10. Concurrency Primitives                                        |          |          |         |    |    |      |     | c6 |
| 4.11. Middleware                                                    |          |          |         |    | c1 |      | c8  |    |
| 4.12. Construction Methods for Distributed and Cloud-Based Software |          | c17, c18 |         |    |    |      |     | c2 |
| 4.13. Constructing Heterogeneous Systems                            |          |          |         |    |    |      | c9  |    |
| 4.14. Performance Analysis and Tuning                               | c25, c26 |          |         |    |    |      |     |    |
| 4.15. Platform Standards                                            |          |          |         | C  |    |      | c10 | c1 |
| 4.16. Test-First Programming                                        | c22      | c8       |         |    |    |      |     |    |
| 4.17. Feedback Loop for Construction                                |          |          | c3, c16 |    |    |      |     |    |
| 5. Software Construction Tools                                      |          |          |         |    |    |      |     |    |
| 5.1. Development Environments                                       | c30      |          |         |    |    |      |     |    |
| 5.2. Visual Programming and Low-Code/Zero- Code Platforms           | c30      |          |         |    |    |      |     |    |
| 5.3. Unit Testing Tools                                             | c22      | c8       |         |    |    |      |     |    |
| 5.4. Profiling, Performance Analysis and Slicing Tools              | c25, c26 |          |         |    |    |      |     |    |

## FURTHER READINGS

- [2] I. Sommerville, Software Engineering, 10th edition, Addison-Wesley, 2016.

IEEE Std. 1517-2010: IEEE Standard for Information Technology — Software Life Cycle [3] G. Kim et al., The DevOps Handbook: Processes — Reuse Processes, IEEE, 1999 [8].

This standard specifies the processes, activities, and tasks to be applied during each phase of the software life cycle to enable a software product to be constructed from reusable assets. It covers the concept of reuse-based development and the processes of construction for reuse and construction with reuse.

ISO/IEC 12207:2008: Information Technology Soye roae Pt ie Pyoos IEC, 2008 [9].

- How to Create World-Class Agility, Reliability &amp; Security in Technology Organizations, 2nd edition, IT Revolution, 2021.
- e[4] H. Heitkötter, S. Hanschke, and T.A. Majchrzak, Evaluating Cross-Platform Development Approaches for Mobile Applications, 2013, in Cordeiro, J., Krempels, K.H. (eds.), Web Information Systems and Technologies. WEBIST 2012. Lecture Notes in Business Information Processing, vol. 140, Springer, Berlin, Heidelberg.

This standard defines a series of software [5] P. Clements et al., Documenting Software development processes, including software Architectures: Views and Beyond, 2nd ediconstruction process, software integration tion, Boston: Pearson Education, 2010. process, and software reuse process.

Martin Fowler, Kent Beck. Refactoring: Improving the Design of Existing Code (2nd Edition), Addison-Wesley Signature Series (Fowler).

Robert C. Martin. Clean Code: A Handbook of Agile Software Craftsmanship, Pearson Education, Inc.

## REFERENCES

- [1] S. McConnell, Code Complete, 2nd edition, Redmond, WA: Microsoft Press, 2004.
- [6] E. Gamma et al., Design Patterns: Elements of Reusable Object-Oriented Software, 1st edition, Reading, MA: Addison-Wesley Professional, 1994.
- [7] S.J. Mellor and M.J. Balcer, Executable UML: A Foundation for Model-Driven Architecture, 1st edition, Boston: Addison-Wesley, 2002.
4. [8*] L. Null and J. Lobur, The Essentials of Computer Organization and Architecture, 5th ed., Jones and Bartlett Publishers, 2018.
- [9] A. Silberschatz et al., Operating System Concepts, 8th edition, Hoboken, NJ: Wiley, 2008.